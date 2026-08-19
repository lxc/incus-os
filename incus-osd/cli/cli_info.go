package cli

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	incusapi "github.com/lxc/incus/v7/shared/api"
	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/lxc/incus/v7/shared/units"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// Info.
type cmdAdminOSInfo struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSInfo) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("info")
	cmd.Short = "Show system information"

	cmd.Long = cli.FormatSection("Description", "Show system information")
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSInfo) get(remote string, endpoint string, data any) error {
	u, err := url.Parse("/os/1.0" + endpoint)
	if err != nil {
		return err
	}

	values := u.Query()
	if c.os.flagTarget != "" {
		values.Set("target", c.os.flagTarget)
	}

	u.RawQuery = values.Encode()

	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", u.String(), nil, nil, "")
	if err != nil {
		return err
	}

	return resp.MetadataAsStruct(data)
}

func (c *cmdAdminOSInfo) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 0, 1)
	if exit {
		return err
	}

	// Parse remote.
	remote := ""
	if len(args) > 0 {
		remote, _ = parseRemote(args[0])
	}

	// Get the general system information.
	var root struct {
		Environment map[string]any `json:"environment"`
	}

	err = c.get(remote, "", &root)
	if err != nil {
		return err
	}

	// Get the security information.
	var security api.SystemSecurity

	err = c.get(remote, "/system/security", &security)
	if err != nil {
		return err
	}

	// Get the network information.
	var network api.SystemNetwork

	err = c.get(remote, "/system/network", &network)
	if err != nil {
		return err
	}

	// Get the system resources.
	var resources incusapi.Resources

	err = c.get(remote, "/system/resources", &resources)
	if err != nil {
		return err
	}

	// Get the list of applications.
	var appURLs []string

	err = c.get(remote, "/applications", &appURLs)
	if err != nil {
		return err
	}

	// Get the incus-osd log entries for the current boot.
	// Request more entries than displayed as the systemd unit messages get filtered out below.
	var logEntries []map[string]any

	err = c.get(remote, "/debug/log?unit=incus-osd.service&boot=0&entries=100", &logEntries)
	if err != nil {
		return err
	}

	// Only keep actual incus-osd messages (skip the systemd unit messages).
	logMessages := []string{}

	for _, entry := range logEntries {
		if entry["SYSLOG_IDENTIFIER"] != "incus-osd" {
			continue
		}

		message, ok := entry["MESSAGE"].(string)
		if !ok {
			continue
		}

		logMessages = append(logMessages, message)
	}

	// Only show the last 25 entries.
	if len(logMessages) > 25 {
		logMessages = logMessages[len(logMessages)-25:]
	}

	// Render the general system information.
	_, _ = fmt.Printf("Hostname: %v\n", root.Environment["hostname"])                             //nolint:forbidigo
	_, _ = fmt.Printf("OS: %v %v\n", root.Environment["os_name"], root.Environment["os_version"]) //nolint:forbidigo
	uptime := strings.TrimSpace(fmt.Sprintf("%v", root.Environment["uptime"]))

	seconds, ok := root.Environment["uptime"].(float64)
	if ok {
		uptime = (time.Duration(seconds) * time.Second).String()
	}

	_, _ = fmt.Printf("Uptime: %s\n", uptime) //nolint:forbidigo

	// Render the machine information.
	if len(resources.CPU.Sockets) > 0 {
		numCores := 0
		for _, socket := range resources.CPU.Sockets {
			numCores += len(socket.Cores)
		}

		_, _ = fmt.Printf("Processor: %s (numa=%d, sockets=%d, cores=%d, threads=%d) (%s)\n", resources.CPU.Sockets[0].Name, len(resources.Memory.Nodes), len(resources.CPU.Sockets), numCores, resources.CPU.Total, resources.CPU.Architecture) //nolint:forbidigo
	}

	_, _ = fmt.Printf("Memory: %s\n", units.GetByteSizeStringIEC(int64(resources.Memory.Total), 0)) //nolint:forbidigo,gosec

	// Render the network addresses.
	_, _ = fmt.Print("\nNetwork:\n") //nolint:forbidigo

	names := make([]string, 0, len(network.State.Interfaces))
	for name := range network.State.Interfaces {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		iface := network.State.Interfaces[name]
		if len(iface.Addresses) == 0 {
			continue
		}

		_, _ = fmt.Printf("  %s: %s\n", name, strings.Join(iface.Addresses, ", ")) //nolint:forbidigo
	}

	// Render the applications.
	if len(appURLs) > 0 {
		_, _ = fmt.Print("\nApplications:\n") //nolint:forbidigo

		for _, appURL := range appURLs {
			name := appURL[strings.LastIndex(appURL, "/")+1:]

			var app api.Application

			err = c.get(remote, "/applications/"+name, &app)
			if err != nil {
				return err
			}

			_, _ = fmt.Printf("  %s (%s)\n", name, app.State.FriendlyVersion) //nolint:forbidigo
		}
	}

	// Render the incus-osd log.
	if len(logMessages) > 0 {
		_, _ = fmt.Print("\nLog:\n") //nolint:forbidigo

		for _, message := range logMessages {
			_, _ = fmt.Printf("  %s\n", message) //nolint:forbidigo
		}
	}

	// Render any warnings.
	warnings := []string{}

	if security.State.TPMStatus == api.TPMStatusSWTPM {
		warnings = append(warnings, "Degraded security state: no physical TPM found, using swtpm")
	}

	if !security.State.SecureBootEnabled {
		warnings = append(warnings, "Degraded security state: Secure Boot is disabled")
	}

	if !security.State.EncryptionRecoveryKeysRetrieved {
		warnings = append(warnings, "Some encryption recovery keys have not been retrieved yet")
	}

	if len(warnings) > 0 {
		_, _ = fmt.Print("\nWarnings:\n") //nolint:forbidigo

		for _, warning := range warnings {
			_, _ = fmt.Printf("  - %s\n", warning) //nolint:forbidigo
		}
	}

	return nil
}
