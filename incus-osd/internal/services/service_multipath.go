package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// Multipath represents the system Multipath service.
type Multipath struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *Multipath) Get(ctx context.Context) (any, error) {
	// Initialize the WWN list if missing.
	if n.state.Services.Multipath.Config.WWNs == nil {
		n.state.Services.Multipath.Config.WWNs = []string{}
	}

	// Get runtime details if enabled.
	if !n.state.Services.Multipath.Config.Enabled {
		return n.state.Services.Multipath, nil
	}

	pathGroupRegexp := regexp.MustCompile(`policy='(.+)' prio=(.+) status=(.+)$`)

	devices := map[string]api.ServiceMultipathDevice{}

	// Retrieve the details for each WWN.
	for _, wwn := range n.state.Services.Multipath.Config.WWNs {
		// Get the reported status.
		out, err := subprocess.RunCommandContext(ctx, "multipath", "-ll", strings.TrimPrefix(wwn, "0x"))
		if err != nil {
			slog.ErrorContext(ctx, "Couldn't get multipath status", "device", wwn, "err", err)

			continue
		}

		// Check that we have a valid response.
		lines := strings.Split(out, "\n")
		if len(lines) < 3 {
			continue
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

		devices[wwn] = device
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

	// Save the state on return.
	defer n.state.Save(ctx)

	// Disable the service if requested.
	if n.state.Services.Multipath.Config.Enabled && !newState.Config.Enabled {
		err := n.Stop(ctx)
		if err != nil {
			return err
		}
	} else {
		// Update the configuration.
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

	// Generate the WWID list.
	f, err := os.Create("/etc/multipath/wwids")
	if err != nil {
		return err
	}

	defer f.Close()

	err = f.Chmod(0o600)
	if err != nil {
		return err
	}

	for _, wwn := range n.state.Services.Multipath.Config.WWNs {
		_, err = fmt.Fprintf(f, "/%s/\n", strings.TrimPrefix(wwn, "0x"))
		if err != nil {
			return err
		}
	}

	err = f.Close()
	if err != nil {
		return err
	}

	// Ensure the service is running.
	err = systemd.StartUnit(ctx, "multipathd.service")
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
