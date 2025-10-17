package cli

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	cli "github.com/lxc/incus/v6/shared/cmd"
	"github.com/spf13/cobra"
)

// IncusOS management command.
type cmdAdminOS struct {
	args *Args

	flagTarget string
}

func (c *cmdAdminOS) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("os")
	cmd.Short = "Manage IncusOS systems"
	cmd.Long = cli.FormatSection("Description", "Manage IncusOS systems")

	// Applications
	adminOSApplicationCmd := cmdAdminOSApplication{os: c}
	cmd.AddCommand(adminOSApplicationCmd.command())

	// Debug
	adminOSDebugCmd := cmdAdminOSDebug{os: c}
	cmd.AddCommand(adminOSDebugCmd.command())

	// Services
	adminOSServiceCmd := cmdAdminOSService{os: c}
	cmd.AddCommand(adminOSServiceCmd.command())

	// System
	adminOSSystemCmd := cmdAdminOSSystem{os: c}
	cmd.AddCommand(adminOSSystemCmd.command())

	// Show a warning.
	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		_, _ = fmt.Fprint(os.Stderr, "WARNING: The IncusOS API and configuration is subject to change\n\n")
	}

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}

// IncusOS application command.
type cmdAdminOSApplication struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSApplication) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("application")
	cmd.Short = "Manage IncusOS applications"
	cmd.Long = cli.FormatSection("Description", "Manage IncusOS applications")

	// List
	adminOSApplicationListCmd := cmdGenericList{os: c.os, entity: "applications", endpoint: "applications"}
	cmd.AddCommand(adminOSApplicationListCmd.command())

	// Show
	adminOSApplicationShowCmd := cmdGenericShow{os: c.os, entity: "application", entityShort: "application", endpoint: "applications"}
	cmd.AddCommand(adminOSApplicationShowCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}

// IncusOS debug command.
type cmdAdminOSDebug struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSDebug) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("debug")
	cmd.Short = "Debug IncusOS systems"
	cmd.Long = cli.FormatSection("Description", "Debug IncusOS systems")

	// Log
	adminOSDebugLogCmd := cmdAdminOSDebugLog{os: c.os}
	cmd.AddCommand(adminOSDebugLogCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}

// Log.
type cmdAdminOSDebugLog struct {
	os *cmdAdminOS

	flagUnit    string
	flagBoot    string
	flagEntries string
}

func (c *cmdAdminOSDebugLog) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("log")
	cmd.Short = "Get debug log"

	cmd.Long = cli.FormatSection("Description", "Get debug log")
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.Flags().StringVarP(&c.flagUnit, "unit", "u", "", "Unit name``")
	cmd.Flags().StringVarP(&c.flagBoot, "boot", "b", "", "Boot number``")
	cmd.Flags().StringVarP(&c.flagEntries, "entries", "n", "", "Number of entries``")

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugLog) run(cmd *cobra.Command, args []string) error {
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

	// Prepare the URL.
	u, err := url.Parse("/os/1.0/debug/log")
	if err != nil {
		return err
	}

	values := u.Query()
	if c.os.flagTarget != "" {
		values.Set("target", c.os.flagTarget)
	}

	if c.flagUnit != "" {
		values.Set("unit", c.flagUnit)
	}

	if c.flagBoot != "" {
		values.Set("boot", c.flagBoot)
	}

	if c.flagEntries != "" {
		values.Set("entries", c.flagEntries)
	}

	u.RawQuery = values.Encode()

	// Get the log.
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", u.String(), nil, "")
	if err != nil {
		return err
	}

	var data []map[string]any

	err = resp.MetadataAsStruct(&data)
	if err != nil {
		return err
	}

	for _, line := range data {
		// Get and parse the timestamp.
		timeStr, ok := line["__REALTIME_TIMESTAMP"].(string)
		if !ok {
			continue
		}

		timeInt, err := strconv.ParseInt(timeStr, 10, 64)
		if err != nil {
			continue
		}

		ts := time.UnixMicro(timeInt)

		// Get the section identifier.
		section, ok := line["SYSLOG_IDENTIFIER"].(string)
		if !ok {
			continue
		}

		// Get the message itself.
		message, ok := line["MESSAGE"].(string)
		if !ok {
			continue
		}

		_, _ = fmt.Printf("[%s] %s: %s\n", ts.Format(dateLayoutSecond), section, message) //nolint:forbidigo
	}

	return nil
}

// IncusOS service command.
type cmdAdminOSService struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSService) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("service")
	cmd.Short = "Manage IncusOS services"
	cmd.Long = cli.FormatSection("Description", "Manage IncusOS services")

	// Edit
	adminOSServiceEditCmd := cmdGenericEdit{os: c.os, entity: "service", entityShort: "service", endpoint: "services"}
	cmd.AddCommand(adminOSServiceEditCmd.command())

	// List
	adminOSApplicationListCmd := cmdGenericList{os: c.os, entity: "services", endpoint: "services"}
	cmd.AddCommand(adminOSApplicationListCmd.command())

	// Show
	adminOSServiceShowCmd := cmdGenericShow{os: c.os, entity: "service", entityShort: "service", endpoint: "services"}
	cmd.AddCommand(adminOSServiceShowCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}

// IncusOS system command.
type cmdAdminOSSystem struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSSystem) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("system")
	cmd.Short = "Manage IncusOS system details"
	cmd.Long = cli.FormatSection("Description", "Manage IncusOS system details")

	// Edit
	adminOSSystemEditCmd := cmdGenericEdit{os: c.os, entity: "system", entityShort: "section", endpoint: "system"}
	cmd.AddCommand(adminOSSystemEditCmd.command())

	// List
	adminOSSystemListCmd := cmdGenericList{os: c.os, entity: "system configuration sections", endpoint: "system"}
	cmd.AddCommand(adminOSSystemListCmd.command())

	// Show
	adminOSSystemShowCmd := cmdGenericShow{os: c.os, entity: "system configuration", entityShort: "section", endpoint: "system"}
	cmd.AddCommand(adminOSSystemShowCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}
