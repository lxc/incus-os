package cli

import (
	cli "github.com/lxc/incus/v6/shared/cmd"
	"github.com/spf13/cobra"
)

// IncusOS application command.
type cmdAdminOSApplication struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSApplication) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("application")
	cmd.Short = "Manage IncusOS applications"
	cmd.Long = cli.FormatSection("Description", "Manage IncusOS applications")

	// Add.
	addCmd := cmdGenericRun{
		os:          c.os,
		action:      "add",
		description: "Add an application",
		endpoint:    "applications",
		hasData:     true,
	}
	cmd.AddCommand(addCmd.command())

	// Backup.
	backupCmd := cmdGenericRun{
		os:            c.os,
		action:        "backup",
		description:   "Backup the application",
		endpoint:      "applications",
		entity:        "application",
		hasData:       true,
		defaultData:   "{}",
		hasFileOutput: true,
	}
	cmd.AddCommand(backupCmd.command())

	// Factory reset.
	factoryResetCmd := cmdGenericRun{
		os:          c.os,
		action:      "factory-reset",
		description: "Factory reset the application",
		endpoint:    "applications",
		entity:      "application",
		hasData:     true,
		defaultData: "{}",
		confirm:     "factory-reset the application",
	}
	cmd.AddCommand(factoryResetCmd.command())

	// List.
	listCmd := cmdGenericList{os: c.os, entity: "applications", endpoint: "applications"}
	cmd.AddCommand(listCmd.command())

	// Restart.
	restartCmd := cmdGenericRun{
		os:          c.os,
		action:      "restart",
		description: "Restart the application",
		endpoint:    "applications",
		entity:      "application",
		confirm:     "restart the application",
	}
	cmd.AddCommand(restartCmd.command())

	// Restore.
	restoreCmd := cmdGenericRun{
		os:           c.os,
		action:       "restore",
		description:  "Restore an application backup",
		endpoint:     "applications",
		entity:       "application",
		hasFileInput: true,
		confirm:      "restore the system state to provided backup",
	}
	cmd.AddCommand(restoreCmd.command())

	// Show.
	showCmd := cmdGenericShow{os: c.os, entity: "application", entityShort: "application", endpoint: "applications"}
	cmd.AddCommand(showCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}
