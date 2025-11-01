package cli

import (
	cli "github.com/lxc/incus/v6/shared/cmd"
	"github.com/spf13/cobra"
)

// IncusOS system command.
type cmdAdminOSSystem struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSSystem) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("system")
	cmd.Short = "Manage IncusOS system details"
	cmd.Long = cli.FormatSection("Description", "Manage IncusOS system details")

	// Backup.
	backupCmd := cmdGenericRun{
		os:            c.os,
		action:        "backup",
		description:   "Backup the system",
		endpoint:      "system",
		hasData:       true,
		defaultData:   "{}",
		hasFileOutput: true,
	}
	cmd.AddCommand(backupCmd.command())

	// Check updates.
	checkUpdatesCmd := cmdGenericRun{
		os:          c.os,
		action:      "check",
		name:        "check-update",
		description: "Check for updates",
		endpoint:    "system/update",
	}
	cmd.AddCommand(checkUpdatesCmd.command())

	// Delete storage pool.
	deleteStoragePoolCmd := cmdGenericRun{
		os:          c.os,
		name:        "delete-storage-pool",
		description: "Delete the storage pool",
		action:      "delete-pool",
		endpoint:    "system/storage",
		hasData:     true,
		confirm:     "delete the storage pool",
	}
	cmd.AddCommand(deleteStoragePoolCmd.command())

	// Edit.
	editCmd := cmdGenericEdit{os: c.os, entity: "system", entityShort: "section", endpoint: "system"}
	cmd.AddCommand(editCmd.command())

	// Factory reset.
	factoryResetCmd := cmdGenericRun{
		os:          c.os,
		action:      "factory-reset",
		description: "Factory reset the system",
		endpoint:    "system",
		hasData:     true,
		defaultData: "{}",
		confirm:     "factory-reset the system",
	}
	cmd.AddCommand(factoryResetCmd.command())

	// Import encryption key.
	importStorageEncryptionKeyCmd := cmdGenericRun{
		os:          c.os,
		name:        "import-storage-encryption-key",
		description: "Import the storage encryption key",
		action:      "import-encryption-key",
		endpoint:    "system/storage",
		hasData:     true,
	}
	cmd.AddCommand(importStorageEncryptionKeyCmd.command())

	// List.
	listCmd := cmdGenericList{os: c.os, entity: "system configuration sections", endpoint: "system"}
	cmd.AddCommand(listCmd.command())

	// Power off.
	poweroffCmd := cmdGenericRun{
		os:          c.os,
		action:      "poweroff",
		description: "Power off the system",
		endpoint:    "system",
		confirm:     "power off the system",
	}
	cmd.AddCommand(poweroffCmd.command())

	// Reboot.
	rebootCmd := cmdGenericRun{
		os:          c.os,
		action:      "reboot",
		description: "Reboot the system",
		endpoint:    "system",
		confirm:     "reboot the system",
	}
	cmd.AddCommand(rebootCmd.command())

	// Restore.
	restoreCmd := cmdGenericRun{
		os:           c.os,
		action:       "restore",
		description:  "Restore a system backup",
		endpoint:     "system",
		hasFileInput: true,
		confirm:      "restore the system state to provided backup",
	}
	cmd.AddCommand(restoreCmd.command())

	// Show.
	showCmd := cmdGenericShow{os: c.os, entity: "system configuration", entityShort: "section", endpoint: "system"}
	cmd.AddCommand(showCmd.command())

	// TPM rebind.
	tpmRebindCmd := cmdGenericRun{
		os:          c.os,
		action:      "tpm-rebind",
		description: "Rebind the TPM (after using recovery key)",
		endpoint:    "system/security",
	}
	cmd.AddCommand(tpmRebindCmd.command())

	// Wipe drive.
	wipeDriveCmd := cmdGenericRun{
		os:          c.os,
		action:      "wipe-drive",
		description: "Wipe the drive",
		endpoint:    "system/storage",
		hasData:     true,
		confirm:     "wipe the drive",
	}
	cmd.AddCommand(wipeDriveCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}
