package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/incus/v7/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// Multipath represents the system Multipath service.
type Multipath struct {
	common

	state *state.State
}

// multipathTransports maps the transports supported for automatic multipathing to their multipath protocol.
var multipathTransports = map[string]string{
	"fc":    "scsi:fcp",
	"iscsi": "scsi:iscsi",
}

// multipathWWNPrefixRegexp matches a valid WWN prefix, nothing else may reach the multipath configuration.
var multipathWWNPrefixRegexp = regexp.MustCompile(`^0x[0-9a-f]+$`)

// Get returns the current service state.
func (n *Multipath) Get(ctx context.Context) (any, error) {
	// Initialize the lists if missing.
	if n.state.Services.Multipath.Config.WWNs == nil {
		n.state.Services.Multipath.Config.WWNs = []string{}
	}

	if n.state.Services.Multipath.Config.AutomaticTransports == nil {
		n.state.Services.Multipath.Config.AutomaticTransports = []string{}
	}

	if n.state.Services.Multipath.Config.AutomaticWWNPrefixes == nil {
		n.state.Services.Multipath.Config.AutomaticWWNPrefixes = []string{}
	}

	// Retrieve the Fibre Channel controller details.
	n.state.Services.Multipath.State.Controllers = n.fcControllers()

	// Get runtime details if enabled.
	if !n.state.Services.Multipath.Config.Enabled {
		n.state.Services.Multipath.State.Devices = map[string]api.ServiceMultipathDevice{}

		return n.state.Services.Multipath, nil
	}

	// Map the WWN of each device to the name multipath knows it by.
	targets := map[string]string{}

	for _, wwn := range n.state.Services.Multipath.Config.WWNs {
		targets[wwn] = "/dev/disk/by-id/wwn-" + wwn
	}

	if len(n.state.Services.Multipath.Config.AutomaticTransports) > 0 {
		// Automatically configured devices aren't part of the configuration, list them all.
		out, err := subprocess.RunCommandContext(ctx, "multipathd", "show", "maps", "raw", "format", "%w")
		if err != nil {
			return nil, err
		}

		for wwid := range strings.FieldsSeq(out) {
			// A leading "3" marks an NAA identifier, which is what the WWN is.
			wwn, ok := strings.CutPrefix(wwid, "3")
			if ok {
				wwn = "0x" + wwn
			}

			targets[wwn] = wwid
		}
	}

	devices := map[string]api.ServiceMultipathDevice{}

	// Retrieve the details for each device.
	for wwn, target := range targets {
		device, err := n.device(ctx, target)
		if err != nil {
			slog.ErrorContext(ctx, "Couldn't get multipath status", "device", wwn, "err", err)

			continue
		}

		if device != nil {
			devices[wwn] = *device
		}
	}

	n.state.Services.Multipath.State.Devices = devices

	return n.state.Services.Multipath, nil
}

// Update updates the service configuration.
func (n *Multipath) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceMultipath)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceMultipath", req)
	}

	// Validate the request.
	err := n.validate(newState.Config)
	if err != nil {
		return err
	}

	// Save the state on return.
	defer n.state.Save()

	// Disable the service if requested.
	if n.state.Services.Multipath.Config.Enabled && !newState.Config.Enabled {
		err := n.Stop(ctx)
		if err != nil {
			return err
		}

		// Update the configuration after stopping the service.
		n.state.Services.Multipath.Config = newState.Config
	} else {
		// Update the configuration before starting the service.
		n.state.Services.Multipath.Config = newState.Config

		// Enable or reconfigure the service if requested.
		err := n.Start(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the service.
func (n *Multipath) Stop(ctx context.Context) error {
	if !n.state.Services.Multipath.Config.Enabled {
		return nil
	}

	// Remove the configuration.
	err := os.RemoveAll("/etc/multipath")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Reload the multipath configuration.
	_, err = subprocess.RunCommandContext(ctx, "multipath", "-r")
	if err != nil {
		return err
	}

	time.Sleep(time.Second)

	// Stop the multipath service.
	err = systemd.StopUnit(ctx, "multipathd.service", "multipathd.socket")
	if err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (n *Multipath) Start(ctx context.Context) error {
	if !n.state.Services.Multipath.Config.Enabled {
		return nil
	}

	// Create the Multipath config directory if missing.
	err := os.Mkdir("/etc/multipath", 0o700)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	err = os.MkdirAll("/etc/multipath/conf.d", 0o700)
	if err != nil {
		return err
	}

	err = os.WriteFile("/etc/multipath/conf.d/incus-os.conf", []byte(n.config()), 0o600)
	if err != nil {
		return err
	}

	// Create or reset the WWID list.
	f, err := os.Create("/etc/multipath/wwids")
	if err != nil {
		return err
	}

	err = f.Chmod(0o600)
	if err != nil {
		_ = f.Close()

		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	// Add the WWNs.
	for _, wwn := range n.state.Services.Multipath.Config.WWNs {
		target, err := filepath.EvalSymlinks("/dev/disk/by-id/wwn-" + wwn)
		if err != nil {
			slog.Warn("Failed to locate multipath disk", "wwn", wwn)

			continue
		}

		_, err = subprocess.RunCommandContext(ctx, "multipath", "-a", target)
		if err != nil {
			slog.Warn("Failed to add multipath disk", "wwn", wwn, "err", err)

			continue
		}
	}

	// Ensure the service is running.
	err = systemd.StartUnit(ctx, "multipathd.service")
	if err != nil {
		return err
	}

	// Have an already running daemon pick up the configuration.
	_, err = subprocess.RunCommandContext(ctx, "multipathd", "reconfigure")
	if err != nil {
		return err
	}

	// Reload the multipath configuration.
	_, err = subprocess.RunCommandContext(ctx, "multipath", "-r")
	if err != nil {
		return err
	}

	return nil
}

// ShouldStart returns true if the service should be started on boot.
func (n *Multipath) ShouldStart() bool {
	return n.state.Services.Multipath.Config.Enabled
}

// Struct returns the API struct for the Multipath service.
func (*Multipath) Struct() any {
	return &api.ServiceMultipath{}
}

// device returns the reported status of a multipath device, or nil if it has none.
func (*Multipath) device(ctx context.Context, target string) (*api.ServiceMultipathDevice, error) {
	pathGroupRegexp := regexp.MustCompile(`policy='(.+)' prio=(.+) status=(.+)$`)

	out, err := subprocess.RunCommandContext(ctx, "multipath", "-ll", target)
	if err != nil {
		return nil, err
	}

	// Check that we have a valid response.
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		return nil, nil //nolint:nilnil
	}

	header := strings.SplitN(lines[0], " ", 3)
	config := strings.SplitN(lines[1], " ", 2)

	device := api.ServiceMultipathDevice{
		Vendor:     header[2],
		Size:       strings.TrimPrefix(config[0], "size="),
		PathGroups: []api.ServiceMultipathPathGroup{},
	}

	var pathGroup *api.ServiceMultipathPathGroup

	for _, line := range lines[2:] {
		if strings.Contains(line, "policy=") {
			// Dealing with a new path group.
			if pathGroup != nil {
				device.PathGroups = append(device.PathGroups, *pathGroup)
			}

			pathGroup = &api.ServiceMultipathPathGroup{
				Paths: []api.ServiceMultipathPath{},
			}

			// Extract the fields.
			fields := pathGroupRegexp.FindStringSubmatch(line)
			if len(fields) != 4 {
				continue
			}

			priority, err := strconv.ParseUint(fields[2], 10, 64)
			if err != nil {
				continue
			}

			pathGroup.Policy = fields[1]
			pathGroup.Priority = priority
			pathGroup.Status = fields[3]

			continue
		}

		if pathGroup != nil {
			prefix := strings.Split(line, "- ")
			if len(prefix) != 2 {
				continue
			}

			fields := strings.Fields(prefix[1])
			if len(fields) < 2 {
				continue
			}

			pathGroup.Paths = append(pathGroup.Paths, api.ServiceMultipathPath{
				ID:     fields[0],
				Status: fields[len(fields)-1],
			})
		}
	}

	if pathGroup != nil {
		device.PathGroups = append(device.PathGroups, *pathGroup)
	}

	return &device, nil
}

// validate checks a new service configuration.
func (*Multipath) validate(config api.ServiceMultipathConfig) error {
	transports := []string{}

	for _, transport := range config.AutomaticTransports {
		_, ok := multipathTransports[transport]
		if !ok {
			return fmt.Errorf("unsupported automatic transport %q, must be \"fc\" or \"iscsi\"", transport)
		}

		if slices.Contains(transports, transport) {
			return fmt.Errorf("automatic transport %q is listed multiple times", transport)
		}

		transports = append(transports, transport)
	}

	for i, prefix := range config.AutomaticWWNPrefixes {
		if !multipathWWNPrefixRegexp.MatchString(prefix) {
			return fmt.Errorf("invalid automatic WWN prefix %q, must be \"0x\" followed by lowercase hexadecimal", prefix)
		}

		if slices.Contains(config.AutomaticWWNPrefixes[:i], prefix) {
			return fmt.Errorf("automatic WWN prefix %q is listed multiple times", prefix)
		}
	}

	// Without a transport to limit it, a prefix could match local drives.
	if len(config.AutomaticWWNPrefixes) > 0 && len(transports) == 0 {
		return errors.New("automatic WWN prefixes require automatic transports")
	}

	// Automatic multipathing ignores any device that isn't on one of its transports.
	if len(transports) > 0 && len(config.WWNs) > 0 {
		return errors.New("WWNs can't be combined with automatic transports")
	}

	if config.Enabled {
		for _, wwn := range config.WWNs {
			_, err := os.Stat("/dev/disk/by-id/wwn-" + wwn)
			if err != nil {
				return fmt.Errorf("failed to locate WWN %q", wwn)
			}
		}
	}

	return nil
}

// config returns the multipath configuration to apply.
func (n *Multipath) config() string {
	var config strings.Builder

	// Queue I/O while all paths are down, the lock manager fences the host if storage never returns.
	config.WriteString("overrides {\n\tno_path_retry queue\n}\n")

	transports := n.state.Services.Multipath.Config.AutomaticTransports
	if len(transports) == 0 {
		return config.String()
	}

	// Multipath every device on the selected transports, even with a single path, and nothing else.
	config.WriteString("\ndefaults {\n\tfind_multipaths greedy\n}\n\nblacklist {\n\tprotocol \".*\"\n")

	prefixes := n.state.Services.Multipath.Config.AutomaticWWNPrefixes
	if len(prefixes) > 0 {
		config.WriteString("\twwid \".*\"\n")
	}

	config.WriteString("}\n\nblacklist_exceptions {\n")

	for _, transport := range transports {
		config.WriteString("\tprotocol \"^" + multipathTransports[transport] + "$\"\n")
	}

	// A leading "3" marks an NAA identifier, which is what the WWN is.
	for _, prefix := range prefixes {
		config.WriteString("\twwid \"^3" + strings.TrimPrefix(prefix, "0x") + "\"\n")
	}

	config.WriteString("}\n")

	return config.String()
}

// fcControllers returns the details of the local Fibre Channel controllers.
func (*Multipath) fcControllers() []api.ServiceMultipathController {
	controllers := []api.ServiceMultipathController{}

	entries, err := os.ReadDir("/sys/class/fc_host")
	if err != nil {
		return controllers
	}

	for _, entry := range entries {
		path := filepath.Join("/sys/class/fc_host", entry.Name())

		readValue := func(name string) string {
			value, err := os.ReadFile(filepath.Join(path, name))
			if err != nil {
				return ""
			}

			return strings.TrimSpace(string(value))
		}

		controllers = append(controllers, api.ServiceMultipathController{
			FabricName:      readValue("fabric_name"),
			NodeName:        readValue("node_name"),
			PortName:        readValue("port_name"),
			PortState:       readValue("port_state"),
			PortType:        readValue("port_type"),
			Speed:           readValue("speed"),
			SupportedSpeeds: readValue("supported_speeds"),
		})
	}

	return controllers
}
