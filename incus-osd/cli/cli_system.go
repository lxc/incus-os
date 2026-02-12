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
		extraArgs: []cmdGenericRunArgs{
			{
				shortFlag:   "s",
				longFlag:    "skip",
				description: "Comma-separated list of items to skip",
			},
		},
	}
	cmd.AddCommand(restoreCmd.command())

	// Add sub-commands.
	type subCommand struct {
		name          string
		description   string
		isWritable    bool
		extraCommands func() []*cobra.Command
	}

	subCommands := []subCommand{
		{
			name:        "kernel",
			description: "System kernel configuration",
			isWritable:  true,
		},
		{
			name:        "logging",
			description: "System logging",
			isWritable:  true,
		},
		{
			name:        "network",
			description: "Network configuration",
			isWritable:  true,
		},
		{
			name:        "provider",
			description: "Image and management provider",
			isWritable:  true,
		},
		{
			name:        "resources",
			description: "System resources",
			isWritable:  false,
		},
		{
			name:        "security",
			description: "Security configuration",
			isWritable:  true,
			extraCommands: func() []*cobra.Command {
				// TPM rebind.
				tpmRebindCmd := cmdGenericRun{
					os:          c.os,
					action:      "tpm-rebind",
					description: "Rebind the TPM (after using recovery key)",
					endpoint:    "system/security",
				}

				return []*cobra.Command{tpmRebindCmd.command()}
			},
		},
		{
			name:        "storage",
			description: "Storage configuration",
			isWritable:  true,
			extraCommands: func() []*cobra.Command {
				// Create storage volume.
				createVolumeCmd := cmdGenericRun{
					os:          c.os,
					name:        "create-volume",
					description: "Create a storage volume",
					action:      "create-volume",
					endpoint:    "system/storage",
					hasData:     true,
				}

				// Delete storage pool.
				deletePoolCmd := cmdGenericRun{
					os:          c.os,
					name:        "delete",
					description: "Delete the storage pool",
					action:      "delete-pool",
					endpoint:    "system/storage",
					hasData:     true,
					confirm:     "delete the storage pool",
				}

				// Delete storage volume.
				deleteVolumeCmd := cmdGenericRun{
					os:          c.os,
					name:        "delete-volume",
					description: "Delete the storage volume",
					action:      "delete-volume",
					endpoint:    "system/storage",
					hasData:     true,
					confirm:     "delete the storage volume",
				}

				// Encrypt drive.
				encryptDriveCmd := cmdGenericRun{
					os:          c.os,
					action:      "encrypt-drive",
					description: "Encrypt the drive",
					endpoint:    "system/storage",
					hasData:     true,
					confirm:     "wipe and encrypt the drive",
				}

				// Import encrypted drive.
				importEncryptedDriveCmd := cmdGenericRun{
					os:          c.os,
					action:      "import-encrypted-drive",
					description: "Import an existing encrypted drive",
					endpoint:    "system/storage",
					hasData:     true,
				}

				// Import storage pool.
				importPoolCmd := cmdGenericRun{
					os:          c.os,
					name:        "import-storage-pool",
					description: "Import an existing encrypted storage pool",
					action:      "import-pool",
					endpoint:    "system/storage",
					hasData:     true,
				}

				// Wipe drive.
				wipeDriveCmd := cmdGenericRun{
					os:          c.os,
					action:      "wipe-drive",
					description: "Wipe the drive",
					endpoint:    "system/storage",
					hasData:     true,
					confirm:     "wipe the drive",
				}

				// Scrub pool.
				scrubPoolCmd := cmdGenericRun{
					os:          c.os,
					action:      "scrub-pool",
					description: "Scrub the storage pool",
					endpoint:    "system/storage",
					hasData:     true,
				}

				return []*cobra.Command{createVolumeCmd.command(), deletePoolCmd.command(), deleteVolumeCmd.command(), encryptDriveCmd.command(), importEncryptedDriveCmd.command(), importPoolCmd.command(), wipeDriveCmd.command(), scrubPoolCmd.command()}
			},
		},
		{
			name:        "update",
			description: "Update configuration",
			isWritable:  true,
			extraCommands: func() []*cobra.Command {
				// Check updates.
				checkUpdatesCmd := cmdGenericRun{
					os:          c.os,
					action:      "check",
					name:        "check",
					description: "Check for updates",
					endpoint:    "system/update",
				}

				return []*cobra.Command{checkUpdatesCmd.command()}
			},
		},
	}

	for _, sub := range subCommands {
		subCmd := &cobra.Command{}
		subCmd.Use = cli.Usage(sub.name)
		subCmd.Short = sub.description
		subCmd.Long = cli.FormatSection("Description", sub.description)

		if sub.isWritable {
			// Edit.
			editCmd := cmdGenericEdit{os: c.os, endpoint: "system/" + sub.name, entityShort: "configuration"}
			subCmd.AddCommand(editCmd.command())
		}

		// Show.
		showCmd := cmdGenericShow{os: c.os, endpoint: "system/" + sub.name}
		subCmd.AddCommand(showCmd.command())

		cmd.AddCommand(subCmd)

		// Extra commands.
		if sub.extraCommands != nil {
			for _, extraCmd := range sub.extraCommands() {
				subCmd.AddCommand(extraCmd)
			}
		}
	}

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}
