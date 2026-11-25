package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"

	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// IncusOS debug storage command.
type cmdAdminOSDebugStorage struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSDebugStorage) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("storage")
	cmd.Short = "Debug IncusOS storage"
	cmd.Long = cli.FormatSection("Description", "Debug IncusOS storage")

	// Trace.
	traceCmd := cmdAdminOSDebugStorageTrace{os: c.os}
	cmd.AddCommand(traceCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}

// Trace.
type cmdAdminOSDebugStorageTrace struct {
	os *cmdAdminOS

	flagActions  []string
	flagDuration int
}

func (c *cmdAdminOSDebugStorageTrace) command() *cobra.Command {
	cmd := &cobra.Command{}

	cmd.Use = cli.Usage("trace", c.os.remoteUsage()+"<device>")
	cmd.Short = "Trace the I/O of a block device"

	cmd.Long = cli.FormatSection("Description", `Trace the I/O of a block device

The device is either a name such as "sda" or a path under /dev.
The output uses the blktrace format and actions use its category names, such as "discard" or "write".`)
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.Flags().StringSliceVarP(&c.flagActions, "action", "a", nil, "I/O category to trace, can be used multiple times``")
	cmd.Flags().IntVarP(&c.flagDuration, "duration", "t", 10, "Trace duration in seconds, 0 for unlimited``")

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugStorageTrace) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote, device paths may themselves include colons.
	remote := ""
	device := args[0]

	if !strings.HasPrefix(device, "/") {
		remote, device = parseRemote(device)
	}

	u, err := url.Parse("/os/1.0/debug/storage/:trace")
	if err != nil {
		return err
	}

	if c.os.flagTarget != "" {
		values := u.Query()
		values.Set("target", c.os.flagTarget)
		u.RawQuery = values.Encode()
	}

	body, err := json.Marshal(api.DebugStorageTrace{
		Device:   device,
		Actions:  c.flagActions,
		Duration: c.flagDuration,
	})
	if err != nil {
		return err
	}

	_, _, err = doQuery(c.os.args.DoHTTP, remote, "POST", u.String(), string(body), os.Stdout, "")

	return err
}
