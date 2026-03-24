//go:debug x509negativeserial=1

// Package main is used for the incus-osd daemon.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/certs"
	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/install"
	"github.com/lxc/incus-os/incus-osd/internal/keyring"
	"github.com/lxc/incus-os/incus-osd/internal/nftables"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/recovery"
	"github.com/lxc/incus-os/incus-osd/internal/rest"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/services"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/tui"
	"github.com/lxc/incus-os/incus-osd/internal/update"
	"github.com/lxc/incus-os/incus-osd/internal/util"
	"github.com/lxc/incus-os/incus-osd/internal/zfs"
)

var (
	varPath = "/var/lib/incus-os/"
	runPath = "/run/incus-os/"
)

func main() {
	ctx := context.Background()

	// Check privileges.
	if os.Getuid() != 0 {
		tui.EarlyError("incus-osd must be run as root")
		os.Exit(1)
	}

	// Create runtime path if missing.
	err := os.Mkdir(runPath, 0o700)
	if err != nil && !os.IsExist(err) {
		tui.EarlyError(err.Error())
		os.Exit(1)
	}

	// Create storage path if missing.
	err = os.Mkdir(varPath, 0o700)
	if err != nil && !os.IsExist(err) {
		tui.EarlyError(err.Error())
		os.Exit(1)
	}

	// Get persistent state.
	s, err := state.LoadOrCreate(filepath.Join(varPath, "state.txt"))
	if err != nil {
		tui.EarlyError("unable to load state file: " + err.Error())
		os.Exit(1)
	}

	// Ensure custom CA certificates are set, if any.
	if len(s.System.Security.Config.CustomCACerts) > 0 {
		err := util.UpdateSystemCustomCACerts(s.System.Security.Config.CustomCACerts)
		if err != nil {
			tui.EarlyError("unable to configure custom CA certificates: " + err.Error())
			os.Exit(1)
		}
	}

	// Get the OS name and version from /lib/os-release.
	osName, osRelease, err := systemd.GetCurrentRelease(ctx)
	if err != nil {
		tui.EarlyError("unable to get OS name and release: " + err.Error())
		os.Exit(1)
	}

	s.OS.Name = osName
	s.OS.RunningRelease = osRelease

	// Record if the system is relying on a swtpm-backed TPM.
	s.UsingSWTPM = secureboot.GetSWTPMInUse()
	if s.UsingSWTPM {
		err := secureboot.BlowTrustedFuse()
		if err != nil {
			tui.EarlyError("unable to blow security fuse: " + err.Error())
			os.Exit(1)
		}
	}

	// Record if the system has booted with Secure Boot disabled.
	sbEnabled, err := secureboot.Enabled()
	if err != nil {
		tui.EarlyError("unable to check Secure Boot state: " + err.Error())
		os.Exit(1)
	}

	s.SecureBootDisabled = !sbEnabled

	if s.SecureBootDisabled {
		err := secureboot.BlowTrustedFuse()
		if err != nil {
			tui.EarlyError("unable to blow security fuse: " + err.Error())
			os.Exit(1)
		}
	} else {
		inAuditMode, err := secureboot.InAuditMode()
		if err != nil {
			tui.EarlyError("unable to check Secure Boot Audit Mode: " + err.Error())
			os.Exit(1)
		}

		if inAuditMode {
			tui.EarlyError("unable to run while Secure Boot is in Audit Mode")
			os.Exit(1)
		}
	}

	// Perform the install check here, so we don't render the TUI footer during install.
	s.ShouldPerformInstall = install.ShouldPerformInstall()

	// Perform first-boot actions, if needed.
	if !s.OS.SuccessfulBoot && !s.ShouldPerformInstall && s.System.Network.Config == nil {
		err := firstBootActions(ctx)
		if err != nil {
			tui.EarlyError("unable to perform first boot actions: " + err.Error())
			os.Exit(1)
		}
	}

	// Clear the reboot flag on startup.
	s.System.Update.State.NeedsReboot = false

	// Restore a prior good network configuration, if present.
	if s.PriorNetworkConfig != nil {
		s.System.Network.Config = s.PriorNetworkConfig
		s.PriorNetworkConfig = nil
	}

	// Get and start the console TUI.
	tuiApp, err := tui.GetTUI(s)
	if err != nil {
		tui.EarlyError(err.Error())
		os.Exit(1)
	}

	go func() {
		err := tuiApp.Run()
		if err != nil {
			tui.EarlyError(err.Error())
			os.Exit(1)
		}
	}()

	// Prepare a logger.
	logger := slog.New(tui.NewCustomTextHandler(tuiApp))
	slog.SetDefault(logger)

	// Run the daemon.
	err = run(ctx, s)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())

		// Allow time for the error message to be read before the daemon restarts.
		time.Sleep(15 * time.Second)

		os.Exit(1)
	}
}

func firstBootActions(ctx context.Context) error {
	// Clear the "IncusOSInstallComplete" UEFI variable, if it exists.
	_, err := subprocess.RunCommandContext(ctx, "chattr", "-i", "/sys/firmware/efi/efivars/IncusOSInstallComplete-12f075e0-2d07-493d-811a-00920a72c04c")
	if err == nil {
		err := os.Remove("/sys/firmware/efi/efivars/IncusOSInstallComplete-12f075e0-2d07-493d-811a-00920a72c04c")
		if err != nil {
			return err
		}
	}

	// Ensure the system timezone is set properly.
	return setTimezone(ctx)
}

func run(ctx context.Context, s *state.State) error {
	// Verify that the system meets minimum requirements for running IncusOS.
	err := install.CheckSystemRequirements(ctx)
	if err != nil {
		t, tuiErr := tui.GetTUI(nil)
		if tuiErr == nil {
			modal := t.AddModal(s.OS.Name, "system-check")
			modal.Update("System check error: [red]" + err.Error() + "[white]\n" + s.OS.Name + " is unable to run until the problem is resolved.")
		}

		slog.ErrorContext(ctx, "System check error: "+err.Error())

		// If we fail the system requirement check, we'll enter a startup loop with the systemd service
		// constantly trying to restart the daemon. Rather than doing that, just sleep here for an hour
		// so the error message doesn't flicker off and on, then exit and let systemd start us again.
		time.Sleep(1 * time.Hour)

		os.Exit(1) //nolint:revive
	}

	// Warn the user if we failed to read any configuration fields from state.
	if len(s.UnrecognizedFields) > 0 {
		slog.ErrorContext(ctx, "Failed to fully parse existing state; no changes will be written to disk")
	}

	for _, field := range s.UnrecognizedFields {
		slog.WarnContext(ctx, "Failed to parse state field '"+field+"', skipping")
	}

	// Check if we should try to install to a local disk.
	if s.ShouldPerformInstall {
		inst, err := install.NewInstall()
		if err != nil {
			return err
		}

		return inst.DoInstall(ctx, s.OS.Name)
	}

	// Check if we have enough free disk space.
	freeSpace, err := storage.GetFreeSpaceInGiB("/")
	if err != nil {
		return err
	}

	if freeSpace < 1.0 {
		slog.ErrorContext(ctx, fmt.Sprintf("Only %.02fGiB free space available in /, attempting emergency disk cleanup", freeSpace))

		// Clear old journal entries.
		_, err = subprocess.RunCommandContext(ctx, "journalctl", "--vacuum-files=1")
		if err != nil {
			return err
		}

		// Clear anything in /var/cache/.
		cacheEntries, err := os.ReadDir("/var/cache/")
		if err != nil {
			return err
		}

		for _, entry := range cacheEntries {
			err := os.RemoveAll(filepath.Join("/var/cache", entry.Name()))
			if err != nil {
				return err
			}
		}
	} else if freeSpace < 5.0 {
		slog.WarnContext(ctx, fmt.Sprintf("Only %.02fGiB free space available in /", freeSpace))
	}

	// Start the API.
	server, err := rest.NewServer(ctx, s, filepath.Join(runPath, "unix.socket"))
	if err != nil {
		return err
	}

	chErr := make(chan error, 1)

	go func() {
		err := server.Serve(ctx)
		chErr <- err
	}()

	// Run startup tasks.
	err = startup(ctx, s)
	if err != nil {
		return err
	}

	// Done with all initialization.
	slog.InfoContext(ctx, "System is ready", "version", s.OS.RunningRelease)
	s.OS.SuccessfulBoot = true

	// Wait for the API to go down.
	return <-chErr
}

func shutdown(ctx context.Context, s *state.State) error {
	// Save state on exit.
	defer func() { _ = s.Save() }()

	t, err := tui.GetTUI(nil)
	if err != nil {
		return err
	}

	modal := t.AddModal("System Shutdown", "shutdown")

	slog.InfoContext(ctx, "System is shutting down", "version", s.OS.RunningRelease)
	modal.Update("System is shutting down")

	// Shutdown the job scheduler.
	err = s.JobScheduler.Shutdown()
	if err != nil {
		return err
	}

	// Run application shutdown actions.
	for appName, appInfo := range s.Applications {
		// Get the application.
		app, err := applications.Load(ctx, s, appName)
		if err != nil {
			return err
		}

		// Stop the application.
		slog.InfoContext(ctx, "Stopping application", "name", appName, "version", appInfo.State.Version)

		err = app.Stop(ctx)
		if err != nil {
			return err
		}
	}

	// Run services shutdown actions (reverse order from startup).
	serviceNames := slices.Clone(services.Supported(s))
	slices.Reverse(serviceNames)

	for _, srvName := range serviceNames {
		srv, err := services.Load(ctx, s, srvName)
		if err != nil {
			return err
		}

		if !srv.ShouldStart() {
			continue
		}

		slog.InfoContext(ctx, "Stopping service", "name", srvName)

		err = srv.Stop(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Failed stopping service", "name", srvName, "err", err)
		}
	}

	return nil
}

func startup(ctx context.Context, s *state.State) error { //nolint:revive
	// Save state on exit.
	defer func() { _ = s.Save() }()

	// Check kernel keyring.
	slog.DebugContext(ctx, "Getting trusted system keys")

	keys, err := keyring.GetKeys(ctx, keyring.PlatformKeyring)
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return errors.New("invalid Secure Boot environment detected, no platform keys loaded")
	}

	// Determine runtime mode.
	mode := "unsafe"

	embeddedCerts, err := certs.GetEmbeddedCertificates()
	if err != nil {
		return err
	}

	for _, key := range keys {
		// Check if we're using a production signing key.
		if slices.Contains(embeddedCerts.ProductionCertSubjectKeyIDs, key.Fingerprint) {
			mode = "production"
		}

		// Check if we're using a development signing key.
		if mode == "unsafe" && (strings.HasPrefix(key.Description, "mkosi of ") || strings.HasPrefix(key.Description, "TestOS - Secure Boot ")) {
			mode = "dev"
		}

		slog.DebugContext(ctx, "Platform keyring entry", "name", key.Description, "key", key.Fingerprint)
	}

	// If no encryption recovery keys have been defined for the root and swap partitions, generate one before going any further.
	if len(s.System.Security.Config.EncryptionRecoveryKeys) == 0 {
		slog.InfoContext(ctx, "Auto-generating encryption recovery key, this may take a few seconds")

		err := systemd.GenerateRecoveryKeys(ctx, s)
		if err != nil {
			return err
		}

		// If Secure Boot is disabled, when setting the initial encryption recovery key,
		// update the encryption bindings to use both PCRs 4 and 7.
		if s.SecureBootDisabled {
			ukiVersions, err := util.GetUKIVersions()
			if err != nil {
				return err
			}

			err = secureboot.UpdatePCR4Binding(ctx, ukiVersions.CurrentFilepath)
			if err != nil {
				return err
			}
		}
	}

	// Update any existing IncusOS installs that don't have a dedicated recovery key. This migration logic
	// can be removed after September 2026.
	_, err = os.Stat("/var/lib/incus-os/recovery.root.key")
	if err != nil && os.IsNotExist(err) {
		slog.InfoContext(ctx, "Updating encryption recovery key bindings, this may take a few seconds")

		// Get the LUKS partitions.
		luksVolumes, err := util.GetLUKSVolumePartitions(ctx)
		if err != nil {
			return err
		}

		// Check if the TPM can unlock the LUKS volumes.
		_, err = subprocess.RunCommandContext(ctx, "cryptsetup", "luksOpen", "--test-passphrase", luksVolumes["root"], "root")
		if err == nil {
			err := systemd.GenerateRecoveryKeys(ctx, s)
			if err != nil {
				return err
			}
		} else {
			slog.WarnContext(ctx, "Current TPM state cannot unlock LUKS volume, unable to update recovery key bindings")
		}
	}

	// Check if the root and swap partitions include a binding on PCR15. If not, update the LUKS bindings before proceeding.
	// This is required to counter the attack described at https://oddlama.org/blog/bypassing-disk-encryption-with-tpm2-unlock/.
	//
	// Binding to exact values of PCRs 4+7 and a PCR11 policy are insufficient when an attacker has physical access to the system
	// and can create a malicious root partition. Because the system will boot with an unmodified UKI and SecureBoot/TPM state,
	// after the system exits the initrd IncusOS will behave like it's a first boot, but more critically the TPM will be in a
	// known "good" state and happily release its encryption key used by LUKS allowing the attacker to trivially extract the
	// LUKS volume key. They can then undo their malicious changes to the disk, and IncusOS wlll have no idea an attack occurred
	// while the attacker now can decrypt the LUKS volumes at any time to exfiltrate data, mess with the system, etc.
	//
	// We add a binding to an empty PCR15. This PCR is extended when a root LUKS volume is successfully opened in the initrd, so the
	// only time the TPM state could automatically unlock things for us is at the beginning of the initrd. After that point, PCR15
	// will have a different value which cannot be reset, rendering the attack impossible.
	//
	// Performing the check and PCR binding update here catches both fresh installs as well as existing deployments. In either case,
	// the TPM state will allow a single re-bind, after which it will only work in the initrd. At some point after September 2026
	// we can move this logic into the recovery key generation block above so only fresh installs are inspected. systemd v259 did
	// add a TPM2PCRs= option to systemd-repart which would also make life easier.
	luksVolumes, err := util.GetLUKSVolumePartitions(ctx)
	if err != nil {
		return err
	}

	isBoundPCR15, err := storage.LUKSBoundToPCR(ctx, luksVolumes["root"], 15)
	if err != nil {
		return err
	}

	if !isBoundPCR15 {
		slog.InfoContext(ctx, "Upgrading LUKS TPM PCR bindings, this may take a few seconds")

		ukiVersions, err := util.GetUKIVersions()
		if err != nil {
			return err
		}

		err = secureboot.HandleSecureBootKeyChange(ctx, ukiVersions.CurrentFilepath, "")
		if err != nil {
			return err
		}
	}

	// Enable swap, if present. Swap isn't normally activated until after exiting the initrd, and because we're not able to
	// rely on the TPM to automatically unlock that partition, systemd cannot enable swap for us during system boot.
	_, err = os.Stat("/dev/mapper/swap")
	if err != nil && os.IsNotExist(err) {
		_, err := os.Stat("/dev/disk/by-partlabel/swap")
		if err == nil {
			// Unlock the LUKS swap volume.
			_, err := subprocess.RunCommandContext(ctx, "systemd-cryptsetup", "attach", "swap", "/dev/disk/by-partlabel/swap", "/var/lib/incus-os/recovery.swap.key")
			if err != nil {
				slog.WarnContext(ctx, "Unable to decrypt LUKS swap partition")
			} else {
				_, err := subprocess.RunCommandContext(ctx, "swapon", "/dev/mapper/swap")
				if err != nil {
					slog.WarnContext(ctx, "Unable to activate encrypted swap partition")
				}
			}
		}
	}

	// Cleanup any accidentally downloaded manifest files. This code can be removed after April 2026.
	_, err = os.Stat(systemd.SystemExtensionsPath)
	if err == nil {
		files, err := os.ReadDir(systemd.SystemExtensionsPath)
		if err == nil {
			for _, file := range files {
				if strings.HasSuffix(file.Name(), ".manifest.json") {
					_ = os.Remove(filepath.Join(systemd.SystemExtensionsPath, file.Name()))
				}
			}
		}
	}

	// Migrate existing sysext images under /var/lib/extensions/ to /var/lib/incus-os-extensions/<version>/.
	// This code can be removed after April 2026.
	err = os.MkdirAll(systemd.LocalExtensionsPath, 0o700)
	if err != nil {
		return err
	}

	migratedApps := false

	for appName, appInfo := range s.Applications {
		oldPath := filepath.Join(systemd.SystemExtensionsPath, appName+".raw")
		newPath := filepath.Join(systemd.LocalExtensionsPath, appInfo.State.Version, appName+".raw")

		fileStat, err := os.Lstat(oldPath)
		if err != nil {
			return err
		}

		// Skip any symlinked application file that may exist.
		if fileStat.Mode().Type()&fs.ModeSymlink != 0 {
			continue
		}

		// Ensure /var/lib/incus-os-extensions/<version>/ exists.
		err = os.Mkdir(filepath.Join(systemd.LocalExtensionsPath, appInfo.State.Version), 0o700)
		if err != nil && !os.IsExist(err) {
			return err
		}

		// Move the application image and create symlink.
		err = os.Rename(oldPath, newPath)
		if err != nil {
			return err
		}

		err = os.Symlink(newPath, oldPath)
		if err != nil {
			return err
		}

		migratedApps = true
	}

	if migratedApps {
		err := systemd.RefreshExtensions(ctx, s.Applications, &s.OS)
		if err != nil {
			return err
		}
	}

	// Get the machine ID.
	machineID, err := s.MachineID()
	if err != nil {
		machineID = "UNKNOWN"
	}

	slog.InfoContext(ctx, "System is starting up", "mode", mode, "version", s.OS.RunningRelease, "machine-id", strings.TrimSuffix(machineID, "\n"))

	// Display a warning if we're running with a swtpm-backed TPM.
	if s.UsingSWTPM {
		slog.WarnContext(ctx, "Degraded security state: no physical TPM found, using swtpm")
	}

	// Display a warning if Secure Boot is disabled.
	if s.SecureBootDisabled {
		slog.WarnContext(ctx, "Degraded security state: Secure Boot is disabled")
	}

	// Display a warning if we're running from the backup image.
	if s.OS.RunningFromBackup() {
		slog.WarnContext(ctx, "Booted from backup "+s.OS.Name+" image version "+s.OS.RunningRelease)
	}

	// Check for and run recovery logic if present.
	err = recovery.CheckRunRecovery(ctx, s)
	if err != nil {
		// If recovery fails, don't return the error, since that will likely put us into a restart loop,
		// resulting in a soft-brick of the server until the recovery media is removed.
		slog.ErrorContext(ctx, "Recovery failed: "+err.Error())
	}

	// If there's no network configuration in the state, attempt to fetch from the seed info.
	if s.System.Network.Config == nil {
		s.System.Network.Config, err = seed.GetNetwork(ctx)
		if err != nil && !seed.IsMissing(err) {
			return err
		}
	}

	// Record the state of auto-unlocked LUKS devices. With some TPMs this can be slow, so cache the
	// result at startup rather than needing to determine it each time a request arrives via the API.
	s.System.Security.State.EncryptedVolumes, err = systemd.ListEncryptedVolumes(ctx)
	if err != nil {
		return err
	}

	// Sometimes the system may not be able to immediately check the provider for any updates.
	// One such example is when Operations Center is installed and the underlying IncusOS system
	// is registered to it as the provider. We need to wait until the Operations Center
	// application has started, otherwise any update check will fail.
	delayInitialUpdateCheck, err := checkDelayInitialUpdate(ctx, s)
	if err != nil {
		return err
	}

	// Perform network configuration.
	slog.InfoContext(ctx, "Bringing up the network")

	err = nftables.ApplyHwaddrFilters(ctx, s.System.Network.Config)
	if err != nil {
		return err
	}

	err = systemd.ApplyNetworkConfiguration(ctx, s, s.System.Network.Config, 30*time.Second, s.OS.SuccessfulBoot, providers.Refresh, delayInitialUpdateCheck)
	if err != nil {
		return err
	}

	// Configure logging.
	err = systemd.SetSyslog(ctx, s.System.Logging.Config.Syslog)
	if err != nil {
		return err
	}

	// Get the provider.
	var provider string

	var providerConfig map[string]string

	switch mode {
	case "production":
		provider = "images"
	case "dev":
		provider = "local"
	default:
		return errors.New("currently unsupported operating mode")
	}

	if s.System.Provider.Config.Name == "" {
		// Apply the provider seed config (if present).
		providerSeed, err := seed.GetProvider(ctx)
		if err != nil && !seed.IsMissing(err) {
			return errors.New("unable to parse provider seed: " + err.Error())
		}

		if providerSeed != nil {
			s.System.Provider.Config = providerSeed.SystemProviderConfig
		} else {
			s.System.Provider.Config.Name = provider
			s.System.Provider.Config.Config = providerConfig
		}

		// Apply the update seed config (if present).
		updateSeed, err := seed.GetUpdate(ctx, &s.System.Update.Config)
		if err != nil && !seed.IsMissing(err) {
			return errors.New("unable to parse update seed: " + err.Error())
		}

		if updateSeed != nil {
			s.System.Update.Config = updateSeed.SystemUpdateConfig
		}
	}

	p, err := providers.Load(ctx, s)
	if err != nil {
		return err
	}

	if !delayInitialUpdateCheck {
		// Perform an initial blocking check for updates before proceeding.
		update.Checker(ctx, s, p, true, false)
	}

	// Ensure all systemd extensions are applied.
	err = systemd.RefreshExtensions(ctx, s.Applications, &s.OS)
	if err != nil {
		return err
	}

	// Run services startup actions. This must be done before bringing up any storage pools.
	for _, srvName := range services.Supported(s) {
		srv, err := services.Load(ctx, s, srvName)
		if err != nil {
			return err
		}

		if !srv.ShouldStart() {
			continue
		}

		slog.InfoContext(ctx, "Starting service", "name", srvName)

		err = srv.Start(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Failed starting service", "name", srvName, "err", err)
		}
	}

	// Ensure any locally-defined pools are available.
	err = setupLocalStorage(ctx, s)
	if err != nil {
		return err
	}

	// Run application startup actions. Must be done after storage pools are loaded.
	for appName := range s.Applications {
		err := applications.StartInitialize(ctx, s, appName)
		if err != nil {
			return err
		}
	}

	// Run periodic update checks if we have a working provider.
	if p != nil {
		go update.Checker(ctx, s, p, false, false)
	}

	// Handle registration.
	if !s.System.Provider.State.Registered {
		// Reload the provider following application startup (so it can fetch the certificate).
		p, err = providers.Load(ctx, s)
		if err != nil {
			return err
		}

		// Register with the provider.
		err = p.Register(ctx, true)
		if err != nil && !errors.Is(err, providers.ErrRegistrationUnsupported) {
			return err
		}

		if err == nil {
			slog.InfoContext(ctx, "Server registered with the provider")

			s.System.Provider.State.Registered = true
			_ = s.Save()
		}
	}

	// Register background jobs.
	err = registerJobs(s)
	if err != nil {
		return err
	}

	// Start the job scheduler.
	s.JobScheduler.Start()

	// Set up handler for daemon actions.
	s.TriggerReboot = make(chan bool, 1)
	s.TriggerShutdown = make(chan bool, 1)
	s.TriggerSuspend = make(chan bool, 1)
	s.TriggerUpdate = make(chan bool, 1)
	chSignal := make(chan os.Signal, 1)
	signal.Notify(chSignal, unix.SIGTERM)

	go func() {
		action := "exit"

		// Action handler.
	waitSignal:
		select {
		case <-chSignal:
		case <-s.TriggerReboot:
			action = "reboot"
		case <-s.TriggerShutdown:
			action = "shutdown"
		case <-s.TriggerSuspend:
			action = "suspend"

			systemd.RestoreWOLMACAddresses(ctx, s)
			_ = systemd.SystemSuspend(ctx)

			goto waitSignal
		case <-s.TriggerUpdate:
			update.Checker(ctx, s, p, false, true)

			goto waitSignal
		}

		err := shutdown(ctx, s)
		if err != nil {
			slog.ErrorContext(ctx, "Failed shutdown sequence", "err", err)
		}

		switch action {
		case "shutdown":
			systemd.RestoreWOLMACAddresses(ctx, s)
			_ = systemd.SystemPowerOff(ctx)
		case "reboot":
			_ = systemd.SystemReboot(ctx)
		default:
		}

		os.Exit(0) //nolint:revive
	}()

	if delayInitialUpdateCheck {
		// Queue a delayed initial start update check 30 seconds after the system has started up.
		go func() {
			time.Sleep(30 * time.Second)

			update.Checker(ctx, s, p, true, false)
		}()
	}

	return nil
}

func registerJobs(s *state.State) error {
	// Register the ZFS scrub job.
	err := s.JobScheduler.RegisterJob(zfs.PoolScrubJob, s.System.Storage.Config.ScrubSchedule, zfs.ScrubAllPools)
	if err != nil {
		return err
	}

	return nil
}

func checkDelayInitialUpdate(ctx context.Context, s *state.State) (bool, error) {
	// Check if any installed application depends on a delayed update check.
	for appName := range s.Applications {
		app, err := applications.Load(ctx, s, appName)
		if err != nil {
			return false, err
		}

		if app.NeedsLateUpdateCheck() {
			return true, nil
		}
	}

	return false, nil
}

func setTimezone(ctx context.Context) error {
	// Get the network seed.
	config, err := seed.GetNetwork(ctx)
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	// Set the system's timezone from the seed data.
	_, err = subprocess.RunCommandContext(ctx, "timedatectl", "set-timezone", config.Time.Timezone)

	return err
}

func setupLocalStorage(ctx context.Context, s *state.State) error {
	slog.InfoContext(ctx, "Bringing up the local storage")

	err := storage.DecryptDrives(ctx)
	if err != nil {
		return err
	}

	err = zfs.LoadPools(ctx, s)
	if err != nil {
		return err
	}

	return nil
}
