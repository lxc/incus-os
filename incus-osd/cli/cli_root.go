package cli

import (
	"fmt"
	"os"

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

	// Applications.
	applicationCmd := cmdAdminOSApplication{os: c}
	cmd.AddCommand(applicationCmd.command())

	// Debug.
	debugCmd := cmdAdminOSDebug{os: c}
	cmd.AddCommand(debugCmd.command())

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
