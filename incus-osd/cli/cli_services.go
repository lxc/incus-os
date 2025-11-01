package cli

import (
	cli "github.com/lxc/incus/v6/shared/cmd"
	"github.com/spf13/cobra"
)

// IncusOS service command.
type cmdAdminOSService struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSService) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("service")
	cmd.Short = "Manage IncusOS services"
	cmd.Long = cli.FormatSection("Description", "Manage IncusOS services")

	// Edit.
	editCmd := cmdGenericEdit{os: c.os, entity: "service", entityShort: "service", endpoint: "services"}
	cmd.AddCommand(editCmd.command())

	// List.
	listCmd := cmdGenericList{os: c.os, entity: "services", endpoint: "services"}
	cmd.AddCommand(listCmd.command())

	// Reset.
	resetCmd := cmdGenericRun{
		os:          c.os,
		action:      "reset",
		description: "Reset the service",
		endpoint:    "services",
		entity:      "service",
		confirm:     "reset the service",
	}
	cmd.AddCommand(resetCmd.command())

	// Show.
	showCmd := cmdGenericShow{os: c.os, entity: "service", entityShort: "service", endpoint: "services"}
	cmd.AddCommand(showCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}
