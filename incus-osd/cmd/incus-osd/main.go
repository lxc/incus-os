// Package main is used for the incus-osd daemon.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/install"
	"github.com/lxc/incus-os/incus-osd/internal/keyring"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/rest"
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
		_, _ = fmt.Fprintf(os.Stderr, "incus-osd must be run as root")
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
	s, err := state.LoadOrCreate(ctx, filepath.Join(varPath, "state.json"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

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
		slog.Error(err.Error())

		// Sleep for a second to allow output buffers to flush.
		time.Sleep(1 * time.Second)

		os.Exit(1)
	}
}

func run(ctx context.Context, s *state.State, t *tui.TUI) error {
	// Verify that the system meets minimum requirements for running Incus OS.
	err := install.CheckSystemRequirements(ctx)
	if err != nil {
		t.DisplayModal("Incus OS", "System check error: [red]"+err.Error()+"[white]\nIncus OS is unable to run until the problem is resolved.", 0, 0)

		return err
	}

	// Check if we should try to install to a local disk.
	if install.ShouldPerformInstall() {
		// Don't display warning about recovery key during install.
		s.System.Encryption.State.RecoveryKeysRetrieved = true

		inst, err := install.NewInstall(t)
		if err != nil {
			return err
		}

		return inst.DoInstall(ctx)
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
	slog.Info("System is ready", "release", s.RunningRelease)

	return server.Serve(ctx)
}

func shutdown(ctx context.Context, s *state.State, t *tui.TUI) error {
	// Save state on exit.
	defer func() { _ = s.Save(ctx) }()

	slog.Info("System is shutting down", "release", s.RunningRelease)
	t.DisplayModal("System shutdown", "Shutting down the system", 0, 0)

	// Run application shutdown actions.
	for appName, appInfo := range s.Applications {
		// Get the application.
		app, err := applications.Load(ctx, appName)
		if err != nil {
			return err
		}

		// Stop the application.
		slog.Info("Stopping application", "name", appName, "version", appInfo.Version)

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

		slog.Info("Stopping service", "name", srvName)

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

	// Get running release.
	slog.Debug("Getting local OS information")
	runningRelease, err := systemd.GetCurrentRelease(ctx)
	if err != nil {
		return err
	}

	s.RunningRelease = runningRelease

	// Check kernel keyring.
	slog.Debug("Getting trusted system keys")
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
		if key.Fingerprint == "087a9632734ad5a6c860cdff7887437a4239d9c3" {
			mode = "production"
		}

		if mode == "unsafe" && strings.HasPrefix(key.Description, "mkosi of ") {
			mode = "dev"
		}

		slog.Debug("Platform keyring entry", "name", key.Description, "key", key.Fingerprint)
	}

	// If no encryption recovery keys have been defined for the root partition, generate one before going any further.
	if len(s.System.Encryption.Config.RecoveryKeys) == 0 {
		err := systemd.GenerateRecoveryKey(ctx, s)
		if err != nil {
			return err
		}
	}

	slog.Info("System is starting up", "mode", mode, "release", s.RunningRelease)

	// If there's no network configuration in the state, attempt to fetch from the seed info.
	if s.System.Network.Config == nil {
		s.System.Network.Config, err = seed.GetNetwork(ctx, seed.SeedPartitionPath)
		if err != nil && !seed.IsMissing(err) {
			return err
		}
	}

	// Perform network configuration.
	slog.Info("Bringing up the network")
	err = systemd.ApplyNetworkConfiguration(ctx, s.System.Network.Config, 10*time.Second)
	if err != nil {
		return err
	}

	// Get the provider.
	var provider string

	switch mode {
	case "production":
		provider = "github"
	case "dev":
		provider = "local"
	default:
		return errors.New("currently unsupported operating mode")
	}

	p, err := providers.Load(ctx, provider, nil)
	if err != nil {
		if !errors.Is(err, providers.ErrProviderUnavailable) {
			return err
		}

		// Provider is currently unavailable.
		slog.Warn("Update provider is currently unavailable", "provider", provider)
	}

	if p != nil {
		// Perform an initial blocking check for updates before proceeding.
		updateChecker(ctx, s, t, p, true, false)
	}

	// Ensure  the "local" ZFS pool is available.
	slog.Info("Bringing up the local storage")
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

		slog.Info("Starting service", "name", srvName)

		err = srv.Start(ctx)
		if err != nil {
			return err
		}
	}

	// Run application startup actions.
	for appName, appInfo := range s.Applications {
		// Get the application.
		app, err := applications.Load(ctx, appName)
		if err != nil {
			return err
		}

		// Start the application.
		slog.Info("Starting application", "name", appName, "version", appInfo.Version)

		err = app.Start(ctx, appInfo.Version)
		if err != nil {
			return err
		}

		// Run initialization if needed.
		if !appInfo.Initialized {
			slog.Info("Initializing application", "name", appName, "version", appInfo.Version)

			err = app.Initialize(ctx)
			if err != nil {
				return err
			}

			appInfo.Initialized = true
			s.Applications[appName] = appInfo
		}
	}

	// Run periodic update checks if we have a working provider.
	if p != nil {
		go updateChecker(ctx, s, t, p, false, false)
	}

	// Set up handler for shutdown tasks.
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
			slog.Error("Failed shutdown sequence", "err", err)
		}

		switch action {
		case "shutdown":
			_ = systemd.SystemPowerOff(ctx)
		case "reboot":
			_ = systemd.SystemReboot(ctx)
		}

		os.Exit(0) //nolint:revive
	}()

	return nil
}

func updateChecker(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, isStartupCheck bool, isUserRequested bool) {
	persistentModalMessage := ""
	installedOSVersion := s.RunningRelease

	for {
		if persistentModalMessage != "" {
			t.DisplayModal("Incus OS Update", persistentModalMessage, 0, 0)
		}

		// Sleep at the top of each loop, except if we're performing a startup check.
		if !isStartupCheck && !isUserRequested {
			time.Sleep(6 * time.Hour)
		}

		// If user requested, clear cache.
		if isUserRequested {
			err := p.ClearCache(ctx)
			if err != nil && !seed.IsMissing(err) {
				slog.Error(err.Error())

				break
			}
		}

		// Determine what applications to install.
		toInstall := []string{"incus"}

		if len(s.Applications) == 0 && isStartupCheck {
			// Assume first start of the daemon.
			apps, err := seed.GetApplications(ctx, seed.SeedPartitionPath)
			if err != nil && !seed.IsMissing(err) {
				slog.Error(err.Error())

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
		newInstalledOSVersion, err := checkDoOSUpdate(ctx, s, t, p, installedOSVersion, isStartupCheck)
		if err != nil {
			slog.Error(err.Error())
			persistentModalMessage = "[red]Error:[white] " + err.Error()

			continue
		}

		if newInstalledOSVersion != "" {
			installedOSVersion = newInstalledOSVersion
			persistentModalMessage = "Incus OS has been updated to version " + newInstalledOSVersion + ".\nPlease reboot the system to finalize update."
			t.DisplayModal("Incus OS Update", persistentModalMessage, 0, 0)
		}

		// Check for application updates.
		appsUpdated := map[string]string{}
		for _, appName := range toInstall {
			newAppVersion, err := checkDoAppUpdate(ctx, s, t, p, appName, isStartupCheck)
			if err != nil {
				slog.Error(err.Error())
				persistentModalMessage = "[red]Error:[white] " + err.Error()

				break
			}

			if newAppVersion != "" {
				appsUpdated[appName] = newAppVersion
			}
		}

		// Apply the system extensions.
		if len(appsUpdated) > 0 {
			slog.Debug("Refreshing system extensions")
			err = systemd.RefreshExtensions(ctx)
			if err != nil {
				slog.Error(err.Error())
				persistentModalMessage = "[red]Error:[white] " + err.Error()

				continue
			}
		}

		// Notify the applications that they need to update/restart.
		for appName, appVersion := range appsUpdated {
			// Get the application.
			app, err := applications.Load(ctx, appName)
			if err != nil {
				slog.Error(err.Error())
				persistentModalMessage = "[red]Error:[white] " + err.Error()

				continue
			}

			// Reload the application.
			slog.Info("Reloading application", "name", appName, "version", appVersion)

			err = app.Update(ctx, appVersion)
			if err != nil {
				slog.Error(err.Error())
				persistentModalMessage = "[red]Error:[white] " + err.Error()

				continue
			}
		}

		if isStartupCheck || isUserRequested {
			// If running a one-time update, we're done.
			break
		}
	}
}

func checkDoOSUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, installedOSVersion string, isStartupCheck bool) (string, error) {
	slog.Debug("Checking for OS updates")

	update, err := p.GetOSUpdate(ctx)
	if err != nil {
		if errors.Is(err, providers.ErrNoUpdateAvailable) {
			return "", nil
		}

		return "", err
	}

	// Apply the update.
	if update.Version() != s.RunningRelease {
		if !update.IsNewerThan(s.RunningRelease) {
			return "", errors.New("local Incus OS version (" + s.RunningRelease + ") is newer than available update (" + update.Version() + "); skipping")
		}

		// Only apply OS update if it's different from what has been most recently been installed.
		if update.Version() != installedOSVersion {
			// Download the update into place.
			slog.Info("Downloading OS update", "release", update.Version())
			t.DisplayModal("Incus OS Update", "Downloading Incus OS update version "+update.Version(), 0, 0)
			err := update.Download(ctx, systemd.SystemUpdatesPath)
			if err != nil {
				return "", err
			}

			// Apply the update and reboot if first time through loop, otherwise wait for user to reboot system.
			slog.Info("Applying OS update", "release", update.Version())
			t.DisplayModal("Incus OS Update", "Applying Incus OS update version "+update.Version(), 0, 0)
			err = systemd.ApplySystemUpdate(ctx, update.Version(), isStartupCheck)
			if err != nil {
				return "", err
			}

			t.RemoveModal()

			return update.Version(), nil
		}
	} else if isStartupCheck {
		slog.Debug("System is already running latest OS release", "release", s.RunningRelease)
	}

	return "", nil
}

func checkDoAppUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, appName string, isStartupCheck bool) (string, error) {
	slog.Debug("Checking for application updates")

	app, err := p.GetApplication(ctx, appName)
	if err != nil {
		if errors.Is(err, providers.ErrNoUpdateAvailable) {
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
		slog.Info("Downloading application", "application", app.Name(), "release", app.Version())
		t.DisplayModal("Incus OS Update", "Downloading application "+app.Name()+" update "+app.Version(), 0, 0)
		err = app.Download(ctx, systemd.SystemExtensionsPath)
		if err != nil {
			return "", err
		}

		t.RemoveModal()

		// Record newly installed application and save state to disk.
		newAppInfo := s.Applications[app.Name()]
		newAppInfo.Version = app.Version()

		s.Applications[app.Name()] = newAppInfo
		_ = s.Save(ctx)

		return app.Version(), nil
	} else if isStartupCheck {
		slog.Debug("System is already running latest application release", "application", app.Name(), "release", app.Version())
	}

	return "", nil
}
