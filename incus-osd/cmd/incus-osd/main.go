// Package main is used for the incus-osd daemon.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
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
	"github.com/lxc/incus-os/incus-osd/internal/rest"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/services"
	"github.com/lxc/incus-os/incus-osd/internal/state"
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
		_, _ = fmt.Fprint(os.Stderr, "incus-osd must be run as root")
		os.Exit(1)
	}

	// Create runtime path if missing.
	err := os.Mkdir(runPath, 0o700)
	if err != nil && !os.IsExist(err) {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create storage path if missing.
	err = os.Mkdir(varPath, 0o700)
	if err != nil && !os.IsExist(err) {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get persistent state.
	s, err := state.LoadOrCreate(ctx, filepath.Join(varPath, "state.txt"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get the OS name and version from /lib/os-release.
	osName, osRelease, err := systemd.GetCurrentRelease(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	s.OS.Name = osName
	s.OS.RunningRelease = osRelease

	// Perform the install check here, so we don't render the TUI footer during install.
	s.ShouldPerformInstall = install.ShouldPerformInstall()

	// Get and start the console TUI.
	tuiApp, err := tui.NewTUI(s)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	go func() {
		err := tuiApp.Run()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
		slog.ErrorContext(ctx, err.Error())

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

	// Run startup tasks.
	err = startup(ctx, s, t)
	if err != nil {
		return err
	}

	// Start the API.
	server, err := rest.NewServer(ctx, s, filepath.Join(runPath, "unix.socket"))
	if err != nil {
		return err
	}

	// Done with all initialization.
	slog.InfoContext(ctx, "System is ready", "release", s.OS.RunningRelease)

	return server.Serve(ctx)
}

func shutdown(ctx context.Context, s *state.State, t *tui.TUI) error {
	// Save state on exit.
	defer func() { _ = s.Save(ctx) }()

	modal := t.AddModal("System shutdown")

	slog.InfoContext(ctx, "System is shutting down", "release", s.OS.RunningRelease)
	modal.Update("System is shutting down")

	// Run application shutdown actions.
	for appName, appInfo := range s.Applications {
		// Get the application.
		app, err := applications.Load(ctx, appName)
		if err != nil {
			return err
		}

		// Stop the application.
		slog.InfoContext(ctx, "Stopping application", "name", appName, "version", appInfo.Version)

		err = app.Stop(ctx, appInfo.Version)
		if err != nil {
			return err
		}
	}

	// Run services shutdown actions.
	for _, srvName := range services.ValidNames {
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
			return err
		}
	}

	return nil
}

func startup(ctx context.Context, s *state.State, t *tui.TUI) error {
	// Save state on exit.
	defer func() { _ = s.Save(ctx) }()

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

	// Temporary migration logic for pre-existing IncusOS installs that don't have a recovery key set for their swap partition.
	// This should be removed by the end of August, 2025.
	needsSwapKey, err := systemd.SwapNeedsRecoveryKeySet(ctx)
	if err != nil {
		return err
	}

	if needsSwapKey {
		slog.InfoContext(ctx, "Setting encryption recovery key for swap partition, this may take a few seconds")

		err := systemd.SwapSetRecoveryKey(ctx, s.System.Security.Config.EncryptionRecoveryKeys[0])
		if err != nil {
			return err
		}
	}

	slog.InfoContext(ctx, "System is starting up", "mode", mode, "release", s.OS.RunningRelease)

	// Display a warning if we're running from the backup image.
	if s.OS.NextRelease != "" && s.OS.RunningRelease != s.OS.NextRelease {
		slog.WarnContext(ctx, "Booted from backup "+s.OS.Name+" image version "+s.OS.RunningRelease)
	}

	// If there's no network configuration in the state, attempt to fetch from the seed info.
	if s.System.Network.Config == nil {
		s.System.Network.Config, err = seed.GetNetwork(ctx, seed.SeedPartitionPath)
		if err != nil && !seed.IsMissing(err) {
			return err
		}
	}

	// Perform network configuration.
	slog.InfoContext(ctx, "Bringing up the network")

	err = systemd.ApplyNetworkConfiguration(ctx, s, 30*time.Second)
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

	if s.System.Provider.Config.Name != "" {
		provider = s.System.Provider.Config.Name
		providerConfig = s.System.Provider.Config.Config
	} else {
		providerSeed, err := seed.GetProvider(ctx, seed.SeedPartitionPath)
		if err != nil && !seed.IsMissing(err) {
			return err
		}

		if providerSeed != nil {
			provider = providerSeed.Name
			providerConfig = providerSeed.Config
		}

		s.System.Provider.Config.Name = provider
		s.System.Provider.Config.Config = providerConfig
	}

	p, err := providers.Load(ctx, s, provider, providerConfig)
	if err != nil {
		return err
	}

	// Perform an initial blocking check for updates before proceeding.
	updateChecker(ctx, s, t, p, true, false)

	// Ensure  the "local" ZFS pool is available.
	slog.InfoContext(ctx, "Bringing up the local storage")

	err = zfs.ImportOrCreateLocalPool(ctx)
	if err != nil {
		return err
	}

	// Run services startup actions.
	for _, srvName := range services.ValidNames {
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
			return err
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
		err = p.Register(ctx)
		if err != nil && !errors.Is(err, providers.ErrRegistrationUnsupported) {
			return err
		}

		if err == nil {
			slog.InfoContext(ctx, "Server registered with the provider")

			s.System.Provider.State.Registered = true
			_ = s.Save(ctx)
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
	app, err := applications.Load(ctx, appName)
	if err != nil {
		return err
	}

	// Start the application.
	slog.InfoContext(ctx, "Starting application", "name", appName, "version", appInfo.Version)

	err = app.Start(ctx, appInfo.Version)
	if err != nil {
		return err
	}

	// Run initialization if needed.
	if !appInfo.Initialized {
		slog.InfoContext(ctx, "Initializing application", "name", appName, "version", appInfo.Version)

		err = app.Initialize(ctx)
		if err != nil {
			return err
		}

		appInfo.Initialized = true
		s.Applications[appName] = appInfo
	}

	return nil
}

func updateChecker(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, isStartupCheck bool, isUserRequested bool) {
	var modal *tui.Modal

	showModalError := func(msg string, err error) {
		slog.ErrorContext(ctx, msg, "err", err.Error(), "provider", p.Type())

		if modal == nil {
			modal = t.AddModal(s.OS.Name + " Update")
		}

		modal.Update("[red]Error[white] " + msg + ": " + err.Error() + " (provider: " + p.Type() + ")")
	}

	for {
		// Sleep at the top of each loop, except if we're performing a startup check.
		if !isStartupCheck && !isUserRequested {
			time.Sleep(6 * time.Hour)
		}

		// If user requested, clear cache.
		if isUserRequested {
			err := p.ClearCache(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to clear provider cache", "err", err.Error())

				break
			}
		}

		// Check for and apply any Secure Boot key updates before performing any OS or application updates.
		err := checkDoSecureBootCertUpdate(ctx, s, t, p, isStartupCheck)
		if err != nil {
			showModalError("Failed to check for Secure Boot key updates", err)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		// Determine what applications to install.
		toInstall := []string{"incus"}

		if len(s.Applications) == 0 && (isStartupCheck || isUserRequested) {
			// Assume first start of the daemon.
			apps, err := seed.GetApplications(ctx, seed.SeedPartitionPath)
			if err != nil && !seed.IsMissing(err) {
				slog.ErrorContext(ctx, "Failed to get application list", "err", err.Error())

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

		// Check for the latest OS update.
		newInstalledOSVersion, err := checkDoOSUpdate(ctx, s, t, p, isStartupCheck)
		if err != nil {
			showModalError("Failed to check for OS updates", err)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		if newInstalledOSVersion != "" {
			if modal == nil {
				modal = t.AddModal(s.OS.Name + " Update")
			}

			modal.Update(s.OS.Name + " has been updated to version " + newInstalledOSVersion + ".\nPlease reboot the system to finalize update.")

			s.RebootRequired = true
		}

		// Check for application updates.
		appsUpdated := map[string]string{}

		for _, appName := range toInstall {
			newAppVersion, err := checkDoAppUpdate(ctx, s, t, p, appName, isStartupCheck)
			if err != nil {
				showModalError("Failed to check for application updates", err)

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
				showModalError("Failed to refresh system extensions", err)

				if isStartupCheck || isUserRequested {
					break
				}

				continue
			}
		}

		// Notify the applications that they need to update/restart.
		for appName, appVersion := range appsUpdated {
			// Get the application.
			app, err := applications.Load(ctx, appName)
			if err != nil {
				showModalError("Failed to load application", err)

				continue
			}

			// Start/reload the application.
			if !isStartupCheck {
				if app.IsRunning(ctx) {
					slog.InfoContext(ctx, "Reloading application", "name", appName, "version", appVersion)

					err := app.Update(ctx, appVersion)
					if err != nil {
						showModalError("Failed to reload application", err)

						continue
					}
				} else {
					err := startInitializeApplication(ctx, s, appName)
					if err != nil {
						showModalError("Failed to start application", err)

						continue
					}
				}
			}
		}

		if isStartupCheck || isUserRequested {
			// If running a one-time update, we're done.
			break
		}
	}
}

func checkDoOSUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, isStartupCheck bool) (string, error) {
	slog.DebugContext(ctx, "Checking for OS updates")

	if s.RebootRequired {
		slog.DebugContext(ctx, "A reboot of the system is required to finalize a pending update")

		return "", nil
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
	if s.OS.NextRelease != "" && s.OS.RunningRelease != s.OS.NextRelease && s.OS.NextRelease == update.Version() {
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

		err := update.Download(ctx, s.OS.Name, systemd.SystemUpdatesPath, modal.UpdateProgress)
		if err != nil {
			return "", err
		}

		// Hide the progress bar.
		modal.UpdateProgress(0.0)

		// Record the release. Need to do it here, since if the system reboots as part of the
		// update we won't be able to save the state to disk.
		priorNextRelease := s.OS.NextRelease
		s.OS.NextRelease = update.Version()
		_ = s.Save(ctx)

		// Apply the update and reboot if first time through loop, otherwise wait for user to reboot system.
		slog.InfoContext(ctx, "Applying OS update", "release", update.Version())
		modal.Update("Applying " + s.OS.Name + " update version " + update.Version())

		err = systemd.ApplySystemUpdate(ctx, s.System.Security.Config.EncryptionRecoveryKeys[0], update.Version(), isStartupCheck)
		if err != nil {
			s.OS.NextRelease = priorNextRelease
			_ = s.Save(ctx)

			return "", err
		}

		return update.Version(), nil
	} else if isStartupCheck {
		slog.DebugContext(ctx, "System is already running latest OS release", "release", s.OS.RunningRelease)
	}

	return "", nil
}

func checkDoAppUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, appName string, isStartupCheck bool) (string, error) {
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
	if app.Version() != s.Applications[app.Name()].Version {
		if s.Applications[app.Name()].Version != "" && !app.IsNewerThan(s.Applications[app.Name()].Version) {
			return "", errors.New("local application " + app.Name() + " version (" + s.Applications[app.Name()].Version + ") is newer than available update (" + app.Version() + "); skipping")
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

		// Record newly installed application and save state to disk.
		newAppInfo := s.Applications[app.Name()]
		newAppInfo.Version = app.Version()

		s.Applications[app.Name()] = newAppInfo
		_ = s.Save(ctx)

		return app.Version(), nil
	} else if isStartupCheck {
		slog.DebugContext(ctx, "System is already running latest application release", "application", app.Name(), "release", app.Version())
	}

	return "", nil
}

func checkDoSecureBootCertUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, isStartupCheck bool) error {
	slog.DebugContext(ctx, "Checking for Secure Boot key updates")

	if s.RebootRequired {
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
		_ = s.Save(ctx)

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

		// Extract the archive and apply any needed updates.
		tmpDir, err := os.MkdirTemp("/tmp", "incus-os")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		_, err = subprocess.RunCommandContext(ctx, "tar", "-C", tmpDir, "-xf", archiveFilepath)
		if err != nil {
			return err
		}

		err = applyIndividualSecureBootUpdates(ctx, s, t, tmpDir, isStartupCheck)
		if err != nil {
			return err
		}

		// If an EFI variable was updated, we'll either be rebooting automatically or waiting
		// for the user to restart the system before going any further.
		if s.RebootRequired {
			return nil
		}
	}

	slog.DebugContext(ctx, "System Secure Boot keys are up to date")

	// Update state and remove the cached download.
	s.SecureBoot.Version = update.Version()
	s.SecureBoot.FullyApplied = true
	_ = os.Remove(archiveFilepath)

	return nil
}

func applyIndividualSecureBootUpdates(ctx context.Context, s *state.State, t *tui.TUI, updatesDir string, isStartupCheck bool) error {
	availableCerts, err := os.ReadDir(updatesDir)
	if err != nil {
		return err
	}

	// Apply any updates in order: KEK, then db, then dbx.
	for _, certType := range []string{"KEK", "db", "dbx"} {
		existingCerts, err := secureboot.GetCertificatesFromVar(certType)
		if err != nil {
			return fmt.Errorf("failed to read EFI variable '%s'", certType)
		}

		for _, certFile := range availableCerts {
			if !strings.HasPrefix(certFile.Name(), certType+"_") {
				continue
			}

			updateFingerprint := strings.TrimPrefix(certFile.Name(), certType+"_")
			updateFingerprint = strings.TrimSuffix(updateFingerprint, ".auth")

			updateFingerprintBytes, err := hex.DecodeString(updateFingerprint)
			if err != nil {
				return err
			}

			if slices.ContainsFunc(existingCerts, func(c x509.Certificate) bool {
				cFingerprint := sha256.Sum256(c.Raw)

				return bytes.Equal(updateFingerprintBytes, cFingerprint[:])
			}) {
				// This update is already present on the system, so nothing to do.
				continue
			}

			modal := t.AddModal(s.OS.Name + " EFI Variable Update")

			// Apply the key update.
			slog.InfoContext(ctx, "Appending certificate SHA256:"+updateFingerprint+" to EFI variable "+certType)
			modal.Update("Appending certificate SHA256:" + updateFingerprint + " to EFI variable " + certType)

			err = secureboot.AppendEFIVarUpdate(ctx, filepath.Join(updatesDir, certFile.Name()), certType)
			if err != nil {
				if certType != "KEK" {
					slog.ErrorContext(ctx, err.Error())
					modal.Update("[red]ERROR:[white] " + err.Error())

					return err
				}

				slog.WarnContext(ctx, "Failed to automatically apply KEK update, likely because a custom PK is configured")

				continue
			}

			s.RebootRequired = true

			if isStartupCheck {
				slog.InfoContext(ctx, "Successfully updated EFI variable. Automatically rebooting system in five seconds.")
				modal.Update("Successfully updated EFI variable. Automatically rebooting system in five seconds.")

				time.Sleep(5 * time.Second)

				_ = systemd.SystemReboot(ctx)

				time.Sleep(60 * time.Second) // Prevent further system start up in the half second or so before things reboot.
			} else {
				slog.InfoContext(ctx, "Successfully updated EFI variable. A reboot is required to finalize the update.")
				modal.Update("Successfully updated EFI variable. A reboot is required to finalize the update.")
			}

			return nil
		}
	}

	return nil
}
