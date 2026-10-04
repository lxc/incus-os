package cli

import (
	"fmt"
	"os"

	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/spf13/cobra"
)

// IncusOS management command.
type cmdAdminOS struct {
	args *Args

	flagTarget string
}

// remoteUsage returns the usage prefix for the optional remote argument.
func (c *cmdAdminOS) remoteUsage() string {
	if !c.args.SupportsRemote {
		return ""
	}

	return "[<remote>:]"
}

// remoteArg checks the arguments of a command taking only an optional remote and returns the remote name.
func (c *cmdAdminOS) remoteArg(cmd *cobra.Command, args []string) (string, bool, error) {
	maxArgs := 0
	if c.args.SupportsRemote {
		maxArgs = 1
	}

	exit, err := cli.CheckArgs(cmd, args, 0, maxArgs)
	if exit {
		return "", true, err
	}

	if len(args) == 0 {
		return "", false, nil
	}

	remote, _ := parseRemote(args[0])

	return remote, false, nil
}

func (c *cmdAdminOS) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("os")
	cmd.Short = "Manage IncusOS systems"
	cmd.Long = cli.FormatSection("Description", "Manage IncusOS systems")

	// Applications.
	applicationCmd := cmdAdminOSApplication{os: c}
	cmd.AddCommand(applicationCmd.command())

	// Debug.
	debugCmd := cmdAdminOSDebug{os: c}
	cmd.AddCommand(debugCmd.command())

	// Info.
	infoCmd := cmdAdminOSInfo{os: c}
	cmd.AddCommand(infoCmd.command())

	// Services.
	serviceCmd := cmdAdminOSService{os: c}
	cmd.AddCommand(serviceCmd.command())

	// Show.
	showCmd := cmdGenericShow{os: c}
	cmd.AddCommand(showCmd.command())

	// System.
	systemCmd := cmdAdminOSSystem{os: c}
	cmd.AddCommand(systemCmd.command())

	// Show a warning.
	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		_, _ = fmt.Fprint(os.Stderr, "WARNING: The IncusOS API and configuration is subject to change\n\n")
	}

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}
