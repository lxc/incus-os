package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	cli "github.com/lxc/incus/v6/shared/cmd"
	"github.com/spf13/cobra"
)

// IncusOS debug command.
type cmdAdminOSDebug struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSDebug) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("debug")
	cmd.Short = "Debug IncusOS systems"
	cmd.Long = cli.FormatSection("Description", "Debug IncusOS systems")

	// Log.
	logCmd := cmdAdminOSDebugLog{os: c.os}
	cmd.AddCommand(logCmd.command())

	// Processes.
	processesCmd := cmdAdminOSDebugProcesses{os: c.os}
	cmd.AddCommand(processesCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
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
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", u.String(), nil, nil, "")
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

// Processes.
type cmdAdminOSDebugProcesses struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSDebugProcesses) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("processes")
	cmd.Short = "Get the system processes"

	cmd.Long = cli.FormatSection("Description", "Get the system processes")
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugProcesses) run(cmd *cobra.Command, args []string) error {
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
	u, err := url.Parse("/os/1.0/debug/processes")
	if err != nil {
		return err
	}

	// Get the log.
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", u.String(), nil, nil, "")
	if err != nil {
		return err
	}

	var data string

	err = resp.MetadataAsStruct(&data)
	if err != nil {
		return err
	}

	_, err = fmt.Print(data) //nolint:forbidigo
	if err != nil {
		return err
	}

	return nil
}
