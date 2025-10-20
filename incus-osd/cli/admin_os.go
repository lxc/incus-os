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
	applicationCmd := cmdAdminOSApplication{os: c}
	cmd.AddCommand(applicationCmd.command())

	// Debug
	debugCmd := cmdAdminOSDebug{os: c}
	cmd.AddCommand(debugCmd.command())

	// Services
	serviceCmd := cmdAdminOSService{os: c}
	cmd.AddCommand(serviceCmd.command())

	// Show.
	showCmd := cmdGenericShow{os: c}
	cmd.AddCommand(showCmd.command())

	// System
	systemCmd := cmdAdminOSSystem{os: c}
	cmd.AddCommand(systemCmd.command())

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
	logCmd := cmdAdminOSDebugLog{os: c.os}
	cmd.AddCommand(logCmd.command())

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
	editCmd := cmdGenericEdit{os: c.os, entity: "service", entityShort: "service", endpoint: "services"}
	cmd.AddCommand(editCmd.command())

	// List
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

	// Show
	showCmd := cmdGenericShow{os: c.os, entity: "service", entityShort: "service", endpoint: "services"}
	cmd.AddCommand(showCmd.command())

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

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}
