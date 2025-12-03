//go:debug x509negativeserial=1

// Package main is used for the incus-osd daemon.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

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

	// Get the OS name and version from /lib/os-release.
	osName, osRelease, err := systemd.GetCurrentRelease(ctx)
	if err != nil {
		tui.EarlyError("unable to get OS name and release: " + err.Error())
		os.Exit(1)
	}

	s.OS.Name = osName
	s.OS.RunningRelease = osRelease

	// Perform the install check here, so we don't render the TUI footer during install.
	s.ShouldPerformInstall = install.ShouldPerformInstall()

	// If this is the system's first boot, set the timezone.
	if !s.ShouldPerformInstall && s.System.Network.Config == nil {
		err := setTimezone(ctx)
		if err != nil {
			tui.EarlyError("unable to set timezone: " + err.Error())
			os.Exit(1)
		}
	}

	// Clear the reboot flag on startup.
	s.System.Update.State.NeedsReboot = false

	// Get and start the console TUI.
	tuiApp, err := tui.NewTUI(s)
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
	err = run(ctx, s, tuiApp)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())

		// Sleep for a second to allow output buffers to flush.
		time.Sleep(1 * time.Second)

		os.Exit(1)
	}
}

func run(ctx context.Context, s *state.State, t *tui.TUI) error {
	// Verify that the system meets minimum requirements for running IncusOS.
	err := install.CheckSystemRequirements(ctx)
	if err != nil {
		modal := t.AddModal(s.OS.Name, "system-check")
		modal.Update("System check error: [red]" + err.Error() + "[white]\n" + s.OS.Name + " is unable to run until the problem is resolved.")

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
		inst, err := install.NewInstall(t)
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
	err = startup(ctx, s, t)
	if err != nil {
		return err
	}

	// Done with all initialization.
	slog.InfoContext(ctx, "System is ready", "version", s.OS.RunningRelease)
	s.OS.SuccessfulBoot = true

	// Wait for the API to go down.
	return <-chErr
}

func shutdown(ctx context.Context, s *state.State, t *tui.TUI) error {
	// Save state on exit.
	defer func() { _ = s.Save() }()

	modal := t.AddModal("System Shutdown", "shutdown")

	slog.InfoContext(ctx, "System is shutting down", "version", s.OS.RunningRelease)
	modal.Update("System is shutting down")

	// Run application shutdown actions.
	for appName, appInfo := range s.Applications {
		// Get the application.
		app, err := applications.Load(ctx, s, appName)
		if err != nil {
			return err
		}

		// Stop the application.
		slog.InfoContext(ctx, "Stopping application", "name", appName, "version", appInfo.State.Version)

		err = app.Stop(ctx, appInfo.State.Version)
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

func startup(ctx context.Context, s *state.State, t *tui.TUI) error { //nolint:revive
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

	for _, key := range keys {
		if key.Fingerprint == "6cdc880c5df31b18176ddaa3528394aa03791f91" {
			mode = "production"
		}

		if mode == "unsafe" && (strings.HasPrefix(key.Description, "mkosi of ") || strings.HasPrefix(key.Description, "TestOS Secure Boot Key ")) {
			mode = "dev"
		}

		slog.DebugContext(ctx, "Platform keyring entry", "name", key.Description, "key", key.Fingerprint)
	}

	// If no encryption recovery keys have been defined for the root and swap partitions, generate one before going any further.
	if len(s.System.Security.Config.EncryptionRecoveryKeys) == 0 {
		slog.InfoContext(ctx, "Auto-generating encryption recovery key, this may take a few seconds")

		err := systemd.GenerateRecoveryKey(ctx, s)
		if err != nil {
			return err
		}
	}

	// Get the machine ID.
	machineID, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		machineID = []byte("UNKNOWN")
	}

	slog.InfoContext(ctx, "System is starting up", "mode", mode, "version", s.OS.RunningRelease, "machine-id", strings.TrimSuffix(string(machineID), "\n"))

	// Display a warning if we're running from the backup image.
	if s.OS.NextRelease != "" && s.OS.RunningRelease != s.OS.NextRelease {
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
		providerSeed, err := seed.GetProvider(ctx)
		if err != nil && !seed.IsMissing(err) {
			return err
		}

		if providerSeed != nil {
			s.System.Provider.Config.Name = providerSeed.Name
			s.System.Provider.Config.Config = providerSeed.Config
		} else {
			s.System.Provider.Config.Name = provider
			s.System.Provider.Config.Config = providerConfig
		}
	}

	p, err := providers.Load(ctx, s)
	if err != nil {
		return err
	}

	if !delayInitialUpdateCheck {
		// Perform an initial blocking check for updates before proceeding.
		updateChecker(ctx, s, t, p, true, false)
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
	slog.InfoContext(ctx, "Bringing up the local storage")

	err = zfs.LoadPools(ctx, s)
	if err != nil {
		return err
	}

	// Run application startup actions. Must be done after storage pools are loaded.
	for appName := range s.Applications {
		err := startInitializeApplication(ctx, s, appName)
		if err != nil {
			return err
		}
	}

	// Run periodic update checks if we have a working provider.
	if p != nil {
		go updateChecker(ctx, s, t, p, false, false)
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

	// Set up handler for daemon actions.
	s.TriggerReboot = make(chan error, 1)
	s.TriggerShutdown = make(chan error, 1)
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
		case <-s.TriggerUpdate:
			updateChecker(ctx, s, t, p, false, true)

			goto waitSignal
		}

		err := shutdown(ctx, s, t)
		if err != nil {
			slog.ErrorContext(ctx, "Failed shutdown sequence", "err", err)
		}

		switch action {
		case "shutdown":
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

			updateChecker(ctx, s, t, p, true, false)
		}()
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

func startInitializeApplication(ctx context.Context, s *state.State, appName string) error {
	appInfo := s.Applications[appName]

	// Get the application.
	app, err := applications.Load(ctx, s, appName)
	if err != nil {
		return err
	}

	// Start the application.
	slog.InfoContext(ctx, "Starting application", "name", appName, "version", appInfo.State.Version)

	err = app.Start(ctx, appInfo.State.Version)
	if err != nil {
		return err
	}

	// Run initialization if needed.
	if !appInfo.State.Initialized {
		slog.InfoContext(ctx, "Initializing application", "name", appName, "version", appInfo.State.Version)

		err = app.Initialize(ctx)
		if err != nil {
			return err
		}

		appInfo.State.Initialized = true
		s.Applications[appName] = appInfo
	}

	// If the application has a TLS certificate, print its fingerprint so the user can verify it when initially connecting.
	cert, err := app.GetCertificate()
	if err == nil {
		rawFp := sha256.Sum256(cert.Certificate[0])

		slog.InfoContext(ctx, "Application TLS certificate fingerprint", "name", appName, "fingerprint", hex.EncodeToString(rawFp[:]))
	}

	return nil
}

func updateChecker(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, isStartupCheck bool, isUserRequested bool) { //nolint:revive
	showModalError := func(msg string, err error) {
		slog.ErrorContext(ctx, msg, "err", err.Error(), "provider", p.Type())

		updateModal := t.GetModal("update")

		if t.GetModal("update") == nil {
			updateModal = t.AddModal(s.OS.Name+" Update", "update")
		}

		updateModal.Update("[red]Error[white] " + msg + ": " + err.Error() + " (provider: " + p.Type() + ")")
	}

	for {
		// If updates are disabled, skip for an hour.
		if !isUserRequested && s.System.Update.Config.CheckFrequency == "never" {
			if isStartupCheck {
				break
			}

			time.Sleep(time.Hour)

			continue
		}

		// Sleep at the top of each loop, except if we're performing a startup or manual check.
		if !isStartupCheck && !isUserRequested {
			timeSinceCheck := time.Since(s.System.Update.State.LastCheck)

			frequency, err := time.ParseDuration(s.System.Update.Config.CheckFrequency)
			if err != nil {
				// Shouldn't be possible, we validate on update.
				s.System.Update.State.Status = "Failed to parse update frequency"
				slog.ErrorContext(ctx, s.System.Update.State.Status, "err", err.Error())

				break
			}

			// If any maintenance windows are defined, limit the time to sleep to be a minimum
			// of the configured check frequency and the start of the next maintenance window,
			// whichever is shorter.
			for _, window := range s.System.Update.Config.MaintenanceWindows {
				if window.TimeUntilActive() > 0 && window.TimeUntilActive() < frequency {
					frequency = window.TimeUntilActive()
				}
			}

			if timeSinceCheck < frequency {
				// Add one minute to the calculated sleep to protect against an edge case
				// where we try to do an update check right at the start of a maintenance window.
				time.Sleep(frequency - timeSinceCheck + 1*time.Minute)
			}
		}

		// Save when we last performed an update check.
		s.System.Update.State.LastCheck = time.Now()
		s.System.Update.State.Status = "Running update check"

		// Check maintenance window, except if we're performing a startup or manual check.
		if !isStartupCheck && !isUserRequested {
			// Check that we are within a defined maintenance window.
			inMaintenanceWindow := len(s.System.Update.Config.MaintenanceWindows) == 0
			for _, window := range s.System.Update.Config.MaintenanceWindows {
				if window.IsCurrentlyActive() {
					inMaintenanceWindow = true

					break
				}
			}

			if !inMaintenanceWindow {
				s.System.Update.State.Status = "Skipping update check outside of maintenance window(s)"
				slog.InfoContext(ctx, s.System.Update.State.Status)

				continue
			}
		}

		// If user requested, clear cache.
		if isUserRequested {
			err := p.ClearCache(ctx)
			if err != nil {
				s.System.Update.State.Status = "Failed to clear provider cache"
				slog.ErrorContext(ctx, s.System.Update.State.Status, "err", err.Error())

				break
			}
		}

		// Check for and apply any Secure Boot key updates before performing any OS or application updates.
		_, err := checkDownloadUpdate(ctx, s, t, p, "SecureBoot", "", isStartupCheck)
		if err != nil {
			s.System.Update.State.Status = "Failed to check for Secure Boot key updates"
			showModalError(s.System.Update.State.Status, err)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		// Determine what applications to install.
		toInstall := []string{"incus"}

		if len(s.Applications) == 0 && (isStartupCheck || isUserRequested) {
			// Assume first start of the daemon.
			apps, err := seed.GetApplications(ctx)
			if err != nil && !seed.IsMissing(err) {
				s.System.Update.State.Status = "Failed to get application list"
				slog.ErrorContext(ctx, s.System.Update.State.Status, "err", err.Error())

				if isStartupCheck || isUserRequested {
					break
				}

				continue
			}

			if apps != nil {
				// We have valid seed data.
				toInstall = []string{}

				for _, app := range apps.Applications {
					toInstall = append(toInstall, app.Name)
				}
			}
		} else {
			// We have an existing application list.
			toInstall = []string{}

			for name := range s.Applications {
				toInstall = append(toInstall, name)
			}
		}

		// Verify that each application has its dependencies, if any, included in the list of applications.
		for _, appName := range toInstall {
			app, err := applications.Load(ctx, s, appName)
			if err != nil {
				s.System.Update.State.Status = "Failed to check application dependencies"
				showModalError(s.System.Update.State.Status, err)

				break
			}

			for _, dep := range app.GetDependencies() {
				if !slices.Contains(toInstall, dep) {
					toInstall = append(toInstall, dep)
				}
			}
		}

		// Check for application updates.
		appsUpdated := map[string]string{}

		for _, appName := range toInstall {
			newAppVersion, err := checkDownloadUpdate(ctx, s, t, p, "application", appName, isStartupCheck)
			if err != nil {
				s.System.Update.State.Status = "Failed to check for application updates"
				showModalError(s.System.Update.State.Status, err)

				break
			}

			if newAppVersion != "" {
				appsUpdated[appName] = newAppVersion
			}
		}

		// Apply the system extensions.
		if len(appsUpdated) > 0 {
			slog.DebugContext(ctx, "Refreshing system extensions")

			err = systemd.RefreshExtensions(ctx)
			if err != nil {
				s.System.Update.State.Status = "Failed to refresh system extensions"
				showModalError(s.System.Update.State.Status, err)

				if isStartupCheck || isUserRequested {
					break
				}

				continue
			}
		}

		// Check for the latest OS update.
		newInstalledOSVersion, err := checkDownloadUpdate(ctx, s, t, p, "OS", "", isStartupCheck)
		if err != nil {
			s.System.Update.State.Status = "Failed to check for OS updates"
			showModalError(s.System.Update.State.Status, err)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		// Notify the applications that they need to update/restart.
		for appName, appVersion := range appsUpdated {
			// Get the application.
			app, err := applications.Load(ctx, s, appName)
			if err != nil {
				s.System.Update.State.Status = "Failed to load application"
				showModalError(s.System.Update.State.Status, err)

				continue
			}

			// Start/reload the application.
			if !isStartupCheck {
				if app.IsRunning(ctx) {
					slog.InfoContext(ctx, "Reloading application", "name", appName, "version", appVersion)

					err := app.Update(ctx, appVersion)
					if err != nil {
						s.System.Update.State.Status = "Failed to reload application"
						showModalError(s.System.Update.State.Status, err)

						continue
					}
				} else {
					err := startInitializeApplication(ctx, s, appName)
					if err != nil {
						s.System.Update.State.Status = "Failed to start application"
						showModalError(s.System.Update.State.Status, err)

						continue
					}
				}
			}
		}

		updateModal := t.GetModal("update")

		if newInstalledOSVersion != "" {
			if updateModal == nil {
				updateModal = t.AddModal(s.OS.Name+" Update", "update")
			}

			s.System.Update.State.Status = s.OS.Name + " has been updated to version " + newInstalledOSVersion
			updateModal.Update(s.OS.Name + " has been updated to version " + newInstalledOSVersion + ".\nPlease reboot the system to finalize update.")

			s.System.Update.State.NeedsReboot = true
		} else {
			s.System.Update.State.Status = "Update check completed"

			if updateModal != nil {
				updateModal.Done()
			}
		}

		if isStartupCheck || isUserRequested {
			// If running a one-time update, we're done.
			break
		}
	}
}

func checkDownloadUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, updateType string, appName string, isStartupCheck bool) (string, error) {
	s.UpdateMutex.Lock()
	defer s.UpdateMutex.Unlock()

	slog.DebugContext(ctx, "Checking for "+updateType+" updates")

	if s.System.Update.State.NeedsReboot {
		slog.DebugContext(ctx, "A reboot of the system is required to finalize a pending update")

		if updateType == "SecureBoot" {
			return "", nil
		}
	}

	// Get the appropriate update.
	var update providers.CommonUpdate

	var err error

	switch updateType {
	case "SecureBoot":
		update, err = p.GetSecureBootCertUpdate(ctx)
	case "OS":
		update, err = p.GetOSUpdate(ctx)
	case "application":
		update, err = p.GetApplicationUpdate(ctx, appName)
	default:
		return "", errors.New("unrecognized update type '" + updateType + "'")
	}

	if err != nil {
		if errors.Is(err, providers.ErrNoUpdateAvailable) {
			slog.DebugContext(ctx, updateType+" update provider doesn't currently have any update")

			return "", nil
		}

		return "", err
	}

	updateNeeded := false

	// Skip any update that isn't newer than what we are already running.
	switch update.(type) {
	case providers.SecureBootCertUpdate:
		updateNeeded = update.Version() != s.SecureBoot.Version

		if updateNeeded && s.SecureBoot.Version != "" && s.SecureBoot.Version != update.Version() && !update.IsNewerThan(s.SecureBoot.Version) {
			return "", errors.New("installed Secure Boot keys version (" + s.SecureBoot.Version + ") is newer than available update (" + update.Version() + "); skipping")
		}
	case providers.OSUpdate:
		// If we're running from the backup image don't attempt to re-update to a broken version.
		if !s.System.Update.State.NeedsReboot && s.OS.NextRelease != "" && s.OS.RunningRelease != s.OS.NextRelease && s.OS.NextRelease == update.Version() {
			slog.WarnContext(ctx, "Latest "+s.OS.Name+" image version "+s.OS.NextRelease+" has been identified as problematic, skipping update")

			return "", nil
		}

		updateNeeded = update.Version() != s.OS.RunningRelease && update.Version() != s.OS.NextRelease

		if updateNeeded && s.OS.RunningRelease != update.Version() && !update.IsNewerThan(s.OS.RunningRelease) {
			return "", errors.New("local " + s.OS.Name + " version (" + s.OS.RunningRelease + ") is newer than available update (" + update.Version() + "); skipping")
		}
	case providers.ApplicationUpdate:
		updateNeeded = update.Version() != s.Applications[appName].State.Version

		if updateNeeded && s.Applications[appName].State.Version != "" && !update.IsNewerThan(s.Applications[appName].State.Version) {
			return "", errors.New("local application " + appName + " version (" + s.Applications[appName].State.Version + ") is newer than available update (" + update.Version() + "); skipping")
		}
	}

	// Apply the update.
	if updateNeeded {
		return applyUpdate(ctx, s, t, update, updateType, appName, isStartupCheck)
	} else if isStartupCheck {
		_, isApplication := update.(providers.ApplicationUpdate)
		if isApplication {
			slog.DebugContext(ctx, "System is already running latest application version", "application", appName, "version", update.Version())
		} else {
			slog.DebugContext(ctx, "System is already running latest "+updateType+" version", "version", update.Version())
		}
	}

	return "", nil
}

func applyUpdate(ctx context.Context, s *state.State, t *tui.TUI, update providers.CommonUpdate, updateType string, appName string, isStartupCheck bool) (string, error) {
	updateModal := t.GetModal("update")

	if t.GetModal("update") == nil {
		updateModal = t.AddModal(s.OS.Name+" Update", "update")
	}

	// Download the update.
	_, isApplication := update.(providers.ApplicationUpdate)
	if isApplication {
		slog.InfoContext(ctx, "Downloading "+updateType+" update", "application", appName, "version", update.Version())
		updateModal.Update("Downloading " + updateType + " update " + appName + " update " + update.Version())
	} else {
		slog.InfoContext(ctx, "Downloading "+updateType+" update", "version", update.Version())
		updateModal.Update("Downloading " + updateType + " update " + update.Version())
	}

	targetPath := ""

	switch update.(type) {
	case providers.SecureBootCertUpdate:
		targetPath = "/tmp/"
	case providers.OSUpdate:
		targetPath = systemd.SystemUpdatesPath
	case providers.ApplicationUpdate:
		targetPath = systemd.SystemExtensionsPath
	}

	err := update.Download(ctx, targetPath, updateModal.UpdateProgress)
	if err != nil {
		return "", err
	}

	// Hide the progress bar.
	updateModal.UpdateProgress(0.0)

	switch u := update.(type) {
	case providers.SecureBootCertUpdate:
		slog.InfoContext(ctx, "Applying Secure Boot certificate update", "version", update.Version())
		updateModal.Update("Applying Secure Boot certificate update version " + update.Version())

		// Immediately set FullyApplied to false and save state to disk.
		s.SecureBoot.FullyApplied = false
		_ = s.Save()

		needsReboot, err := secureboot.UpdateSecureBootCerts(ctx, filepath.Join(targetPath, u.GetFilename()))
		if err != nil {
			return "", err
		}

		// If an EFI variable was updated, we'll either be rebooting automatically or waiting
		// for the user to restart the system before going any further.
		if needsReboot {
			updateModal.Done()

			s.System.Update.State.NeedsReboot = true

			sbModal := t.GetModal("secureboot-update")

			if t.GetModal("secureboot-update") == nil {
				sbModal = t.AddModal(s.OS.Name+" SecureBoot Certificate Update", "secureboot-update")
			}

			if isStartupCheck {
				slog.InfoContext(ctx, "Automatically rebooting system in five seconds")
				sbModal.Update("Automatically rebooting system in five seconds")

				time.Sleep(5 * time.Second)

				_ = systemd.SystemReboot(ctx)

				time.Sleep(60 * time.Second) // Prevent further system start up in the half second or so before things reboot.
			} else {
				slog.InfoContext(ctx, "A reboot is required to finalize the update")
				sbModal.Update("A reboot is required to finalize the update")
			}

			return "", nil
		}

		// Update state once all SecureBoot keys are updated.
		s.SecureBoot.Version = update.Version()
		s.SecureBoot.FullyApplied = true
	case providers.OSUpdate:
		// Record the release. Need to do it here, since if the system reboots as part of the
		// update we won't be able to save the state to disk.
		priorNextRelease := s.OS.NextRelease
		s.OS.NextRelease = update.Version()
		_ = s.Save()

		// Apply the update and reboot if first time through loop, otherwise wait for user to reboot system.
		slog.InfoContext(ctx, "Applying OS update", "version", update.Version())
		updateModal.Update("Applying " + s.OS.Name + " update version " + update.Version())

		err = systemd.ApplySystemUpdate(ctx, s.System.Security.Config.EncryptionRecoveryKeys[0], update.Version(), s.System.Update.Config.AutoReboot || isStartupCheck)
		if err != nil {
			s.OS.NextRelease = priorNextRelease
			_ = s.Save()

			return "", err
		}

		// Record the state of auto-unlocked LUKS devices. With some TPMs this can be slow, so cache the
		// result after applying an OS update rather than needing to determine it each time a request
		// arrives via the API.
		s.System.Security.State.EncryptedVolumes, err = systemd.ListEncryptedVolumes(ctx)
		if err != nil {
			s.OS.NextRelease = priorNextRelease
			_ = s.Save()

			return "", err
		}
	case providers.ApplicationUpdate:
		// Verify the application is signed with a trusted key in the kernel's keyring.
		err = systemd.VerifyExtensionCertificateFingerprint(ctx, filepath.Join(systemd.SystemExtensionsPath, appName+".raw"))
		if err != nil {
			return "", err
		}

		// Record newly installed application and save state to disk.
		newAppInfo := s.Applications[appName]
		newAppInfo.State.Version = update.Version()

		s.Applications[appName] = newAppInfo
		_ = s.Save()
	}

	return update.Version(), nil
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
