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

	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/install"
	"github.com/lxc/incus-os/incus-osd/internal/keyring"
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

var updateModal *tui.Modal

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

	// If the update frequency is set to less than five minutes, reset to a default of six hours.
	if s.System.Update.Config.UpdateFrequency.Minutes() < 5 {
		s.System.Update.Config.UpdateFrequency = 6 * time.Hour
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
	// Verify that the system meets minimum requirements for running Incus OS.
	err := install.CheckSystemRequirements(ctx)
	if err != nil {
		modal := t.AddModal(s.OS.Name)
		modal.Update("System check error: [red]" + err.Error() + "[white]\n" + s.OS.Name + " is unable to run until the problem is resolved.")

		slog.ErrorContext(ctx, "System check error: "+err.Error())

		// If we fail the system requirement check, we'll enter a startup loop with the systemd service
		// constantly trying to restart the daemon. Rather than doing that, just sleep here for an hour
		// so the error message doesn't flicker off and on, then exit and let systemd start us again.
		time.Sleep(1 * time.Hour)

		os.Exit(1) //nolint:revive
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
	slog.InfoContext(ctx, "System is ready", "release", s.OS.RunningRelease)

	// Wait for the API to go down.
	return <-chErr
}

func shutdown(ctx context.Context, s *state.State, t *tui.TUI) error {
	// Save state on exit.
	defer func() { _ = s.Save() }()

	modal := t.AddModal("System shutdown")

	slog.InfoContext(ctx, "System is shutting down", "release", s.OS.RunningRelease)
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

func startup(ctx context.Context, s *state.State, t *tui.TUI) error {
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

	slog.InfoContext(ctx, "System is starting up", "mode", mode, "release", s.OS.RunningRelease, "machine-id", strings.TrimSuffix(string(machineID), "\n"))

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
		s.System.Network.Config, err = seed.GetNetwork(ctx, seed.GetSeedPath())
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

	// Perform network configuration.
	slog.InfoContext(ctx, "Bringing up the network")

	err = systemd.ApplyNetworkConfiguration(ctx, s, s.System.Network.Config, 30*time.Second, providers.Refresh)
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
		providerSeed, err := seed.GetProvider(ctx, seed.GetSeedPath())
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

	// Perform an initial blocking check for updates before proceeding.
	updateChecker(ctx, s, t, p, true, false)

	// Ensure any local ZFS pools are available.
	slog.InfoContext(ctx, "Bringing up the local storage")

	err = zfs.LoadPools(ctx, s)
	if err != nil {
		return err
	}

	// Run services startup actions.
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

	// Run application startup actions.
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

	return nil
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

	return nil
}

func updateChecker(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, isStartupCheck bool, isUserRequested bool) { //nolint:revive
	showModalError := func(msg string, err error) {
		slog.ErrorContext(ctx, msg, "err", err.Error(), "provider", p.Type())

		if updateModal == nil {
			updateModal = t.AddModal(s.OS.Name + " Update")
		}

		updateModal.Update("[red]Error[white] " + msg + ": " + err.Error() + " (provider: " + p.Type() + ")")
	}

	for {
		// Sleep at the top of each loop, except if we're performing a startup or manual check.
		if !isStartupCheck && !isUserRequested {
			timeSinceCheck := time.Since(s.System.Update.State.LastCheck)
			if timeSinceCheck < s.System.Update.Config.UpdateFrequency {
				time.Sleep(s.System.Update.Config.UpdateFrequency - timeSinceCheck)
			}
		}

		// Save when we last performed an update check.
		s.System.Update.State.LastCheck = time.Now().UTC()
		s.System.Update.State.UpdateStatus = "Running update check"

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
				s.System.Update.State.UpdateStatus = "Skipping update check outside of maintenance window(s)"
				slog.InfoContext(ctx, s.System.Update.State.UpdateStatus)

				continue
			}
		}

		// If user requested, clear cache.
		if isUserRequested {
			err := p.ClearCache(ctx)
			if err != nil {
				s.System.Update.State.UpdateStatus = "Failed to clear provider cache"
				slog.ErrorContext(ctx, s.System.Update.State.UpdateStatus, "err", err.Error())

				break
			}
		}

		// Check for and apply any Secure Boot key updates before performing any OS or application updates.
		err := checkDoSecureBootCertUpdate(ctx, s, t, p, isStartupCheck)
		if err != nil {
			s.System.Update.State.UpdateStatus = "Failed to check for Secure Boot key updates"
			showModalError(s.System.Update.State.UpdateStatus, err)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		// Determine what applications to install.
		toInstall := []string{"incus"}

		if len(s.Applications) == 0 && (isStartupCheck || isUserRequested) {
			// Assume first start of the daemon.
			apps, err := seed.GetApplications(ctx, seed.GetSeedPath())
			if err != nil && !seed.IsMissing(err) {
				s.System.Update.State.UpdateStatus = "Failed to get application list"
				slog.ErrorContext(ctx, s.System.Update.State.UpdateStatus, "err", err.Error())

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

		// Check for application updates.
		appsUpdated := map[string]string{}

		for _, appName := range toInstall {
			newAppVersion, err := checkDoAppUpdate(ctx, s, t, p, appName, isStartupCheck)
			if err != nil {
				s.System.Update.State.UpdateStatus = "Failed to check for application updates"
				showModalError(s.System.Update.State.UpdateStatus, err)

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
				s.System.Update.State.UpdateStatus = "Failed to refresh system extensions"
				showModalError(s.System.Update.State.UpdateStatus, err)

				if isStartupCheck || isUserRequested {
					break
				}

				continue
			}
		}

		// Check for the latest OS update.
		newInstalledOSVersion, err := checkDoOSUpdate(ctx, s, t, p, isStartupCheck)
		if err != nil {
			s.System.Update.State.UpdateStatus = "Failed to check for OS updates"
			showModalError(s.System.Update.State.UpdateStatus, err)

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
				s.System.Update.State.UpdateStatus = "Failed to load application"
				showModalError(s.System.Update.State.UpdateStatus, err)

				continue
			}

			// Start/reload the application.
			if !isStartupCheck {
				if app.IsRunning(ctx) {
					slog.InfoContext(ctx, "Reloading application", "name", appName, "version", appVersion)

					err := app.Update(ctx, appVersion)
					if err != nil {
						s.System.Update.State.UpdateStatus = "Failed to reload application"
						showModalError(s.System.Update.State.UpdateStatus, err)

						continue
					}
				} else {
					err := startInitializeApplication(ctx, s, appName)
					if err != nil {
						s.System.Update.State.UpdateStatus = "Failed to start application"
						showModalError(s.System.Update.State.UpdateStatus, err)

						continue
					}
				}
			}
		}

		if newInstalledOSVersion != "" {
			if updateModal == nil {
				updateModal = t.AddModal(s.OS.Name + " Update")
			}

			s.System.Update.State.UpdateStatus = s.OS.Name + " has been updated to version " + newInstalledOSVersion
			updateModal.Update(s.OS.Name + " has been updated to version " + newInstalledOSVersion + ".\nPlease reboot the system to finalize update.")

			s.System.Update.State.NeedsReboot = true
		} else {
			s.System.Update.State.UpdateStatus = "Update check completed"
		}

		if isStartupCheck || isUserRequested {
			// If running a one-time update, we're done.
			break
		}
	}
}

func checkDoOSUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, isStartupCheck bool) (string, error) {
	s.UpdateMutex.Lock()
	defer s.UpdateMutex.Unlock()

	slog.DebugContext(ctx, "Checking for OS updates")

	if s.System.Update.State.NeedsReboot {
		slog.DebugContext(ctx, "A reboot of the system is required to finalize a pending update")
	}

	update, err := p.GetOSUpdate(ctx, s.OS.Name)
	if err != nil {
		if errors.Is(err, providers.ErrNoUpdateAvailable) {
			slog.DebugContext(ctx, "OS update provider doesn't currently have any update")

			return "", nil
		}

		return "", err
	}

	// If we're running from the backup image don't attempt to re-update to a broken version.
	if !s.System.Update.State.NeedsReboot && s.OS.NextRelease != "" && s.OS.RunningRelease != s.OS.NextRelease && s.OS.NextRelease == update.Version() {
		slog.WarnContext(ctx, "Latest "+s.OS.Name+" image version "+s.OS.NextRelease+" has been identified as problematic, skipping update")

		return "", nil
	}

	// Skip any update that isn't newer than what we are already running.
	if s.OS.RunningRelease != update.Version() && !update.IsNewerThan(s.OS.RunningRelease) {
		return "", errors.New("local " + s.OS.Name + " version (" + s.OS.RunningRelease + ") is newer than available update (" + update.Version() + "); skipping")
	}

	// Apply the update.
	if update.Version() != s.OS.RunningRelease && update.Version() != s.OS.NextRelease {
		// Download the update into place.
		modal := t.AddModal(s.OS.Name + " Update")
		defer modal.Done()

		slog.InfoContext(ctx, "Downloading OS update", "release", update.Version())
		modal.Update("Downloading " + s.OS.Name + " update version " + update.Version())

		err := update.DownloadUpdate(ctx, s.OS.Name, systemd.SystemUpdatesPath, modal.UpdateProgress)
		if err != nil {
			return "", err
		}

		// Hide the progress bar.
		modal.UpdateProgress(0.0)

		// Record the release. Need to do it here, since if the system reboots as part of the
		// update we won't be able to save the state to disk.
		priorNextRelease := s.OS.NextRelease
		s.OS.NextRelease = update.Version()
		_ = s.Save()

		// Apply the update and reboot if first time through loop, otherwise wait for user to reboot system.
		slog.InfoContext(ctx, "Applying OS update", "release", update.Version())
		modal.Update("Applying " + s.OS.Name + " update version " + update.Version())

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

		return update.Version(), nil
	} else if isStartupCheck {
		slog.DebugContext(ctx, "System is already running latest OS release", "release", s.OS.RunningRelease)
	}

	return "", nil
}

func checkDoAppUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, appName string, isStartupCheck bool) (string, error) {
	s.UpdateMutex.Lock()
	defer s.UpdateMutex.Unlock()

	slog.DebugContext(ctx, "Checking for application updates")

	app, err := p.GetApplication(ctx, appName)
	if err != nil {
		if errors.Is(err, providers.ErrNoUpdateAvailable) {
			slog.DebugContext(ctx, "Application update provider doesn't currently have any update")

			return "", nil
		}

		return "", err
	}

	// Apply the update.
	if app.Version() != s.Applications[app.Name()].State.Version {
		if s.Applications[app.Name()].State.Version != "" && !app.IsNewerThan(s.Applications[app.Name()].State.Version) {
			return "", errors.New("local application " + app.Name() + " version (" + s.Applications[app.Name()].State.Version + ") is newer than available update (" + app.Version() + "); skipping")
		}

		// Download the application.
		modal := t.AddModal(s.OS.Name + " Update")
		defer modal.Done()

		slog.InfoContext(ctx, "Downloading application", "application", app.Name(), "release", app.Version())
		modal.Update("Downloading application " + app.Name() + " update " + app.Version())

		err = app.Download(ctx, systemd.SystemExtensionsPath, modal.UpdateProgress)
		if err != nil {
			return "", err
		}

		// Verify the application is signed with a trusted key in the kernel's keyring.
		err = systemd.VerifyExtensionCertificateFingerprint(ctx, filepath.Join(systemd.SystemExtensionsPath, app.Name()+".raw"))
		if err != nil {
			return "", err
		}

		// Record newly installed application and save state to disk.
		newAppInfo := s.Applications[app.Name()]
		newAppInfo.State.Version = app.Version()

		s.Applications[app.Name()] = newAppInfo
		_ = s.Save()

		return app.Version(), nil
	} else if isStartupCheck {
		slog.DebugContext(ctx, "System is already running latest application release", "application", app.Name(), "release", app.Version())
	}

	return "", nil
}

func checkDoSecureBootCertUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, isStartupCheck bool) error {
	s.UpdateMutex.Lock()
	defer s.UpdateMutex.Unlock()

	slog.DebugContext(ctx, "Checking for Secure Boot key updates")

	if s.System.Update.State.NeedsReboot {
		slog.DebugContext(ctx, "A reboot of the system is required to finalize a pending update")

		return nil
	}

	update, err := p.GetSecureBootCertUpdate(ctx, s.OS.Name)
	if err != nil {
		if errors.Is(err, providers.ErrNoUpdateAvailable) {
			slog.DebugContext(ctx, "Secure Boot key update provider doesn't currently have any update")

			return nil
		}

		return err
	}

	// Skip any update that isn't newer than what we are already running.
	if s.SecureBoot.Version != "" && s.SecureBoot.Version != update.Version() && !update.IsNewerThan(s.SecureBoot.Version) {
		return errors.New("installed Secure Boot keys version (" + s.SecureBoot.Version + ") is newer than available update (" + update.Version() + "); skipping")
	}

	archiveFilepath := filepath.Join(varPath, s.OS.Name+"_SecureBootKeys_"+update.Version()+".tar.gz")

	// Apply the update.
	if update.Version() != s.SecureBoot.Version && !s.SecureBoot.FullyApplied { //nolint:nestif
		// Immediately set FullyApplied to false and save state to disk.
		s.SecureBoot.FullyApplied = false
		_ = s.Save()

		// Check if we need to download the update or not.
		_, err := os.Stat(archiveFilepath)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}

			err := update.Download(ctx, s.OS.Name, varPath)
			if err != nil {
				return err
			}
		}

		modal := t.AddModal(s.OS.Name + " EFI Variable Update")

		slog.InfoContext(ctx, "Applying Secure Boot certificate update version "+update.Version()+".")
		modal.Update("Applying Secure Boot certificate update version " + update.Version() + ".")

		needsReboot, err := secureboot.UpdateSecureBootCerts(ctx, archiveFilepath)
		if err != nil {
			modal.Done()

			return err
		}

		// If an EFI variable was updated, we'll either be rebooting automatically or waiting
		// for the user to restart the system before going any further.
		if needsReboot {
			s.System.Update.State.NeedsReboot = true

			if isStartupCheck {
				slog.InfoContext(ctx, "Automatically rebooting system in five seconds.")
				modal.Update("Automatically rebooting system in five seconds.")

				time.Sleep(5 * time.Second)

				_ = systemd.SystemReboot(ctx)

				time.Sleep(60 * time.Second) // Prevent further system start up in the half second or so before things reboot.
			} else {
				slog.InfoContext(ctx, "A reboot is required to finalize the update.")
				modal.Update("A reboot is required to finalize the update.")
			}

			return nil
		}

		modal.Done()
	}

	slog.DebugContext(ctx, "System Secure Boot keys are up to date")

	// Update state and remove zip file once all SecureBoot keys are updated.
	s.SecureBoot.Version = update.Version()
	s.SecureBoot.FullyApplied = true
	_ = os.Remove(archiveFilepath)

	return nil
}
