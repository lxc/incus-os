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
	logger := slog.New(slog.NewTextHandler(tuiApp, nil))
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
	// Check if we should try to install to a local disk.
	if install.IsInstallNeeded() {
		// Don't display warning about recovery key during install.
		s.System.Encryption.RecoveryKeysRetrieved = true

		inst, err := install.NewInstall(t)
		if err != nil {
			return err
		}

		return inst.DoInstall(ctx)
	}

	// Run startup tasks.
	err := startup(ctx, s, t)
	if err != nil {
		return err
	}

	// Set up handler for shutdown tasks.
	s.TriggerReboot = make(chan error, 1)
	s.TriggerShutdown = make(chan error, 1)
	chSignal := make(chan os.Signal, 1)
	signal.Notify(chSignal, unix.SIGTERM)
	go func() {
		action := "exit"

		// Shutdown handler.
		select {
		case <-chSignal:
		case <-s.TriggerReboot:
			action = "reboot"
		case <-s.TriggerShutdown:
			action = "shutdown"
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

	// Start the API.
	server, err := rest.NewServer(ctx, s, filepath.Join(runPath, "unix.socket"))
	if err != nil {
		return err
	}

	return server.Serve(ctx)
}

func shutdown(ctx context.Context, s *state.State, t *tui.TUI) error {
	// Save state on exit.
	defer func() { _ = s.Save(ctx) }()

	slog.Info("Shutting down", "release", s.RunningRelease)
	t.DisplayModal("System shutdown", "Shutting down the system", 0, 0)

	// Run application shutdown actions..
	for appName, appInfo := range s.Applications {
		// Get the application.
		app, err := applications.Load(ctx, appName)
		if err != nil {
			return err
		}

		// Start the application.
		slog.Info("Stopping application", "name", appName, "version", appInfo.Version)

		err = app.Stop(ctx, appInfo.Version)
		if err != nil {
			return err
		}
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
	slog.Info("Getting local OS information")
	runningRelease, err := systemd.GetCurrentRelease(ctx)
	if err != nil {
		return err
	}

	s.RunningRelease = runningRelease

	// Check kernel keyring.
	slog.Info("Getting trusted system keys")
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
		if key.Fingerprint == "7d4dc2ac7ad1ef27365ff599612e07e2312adf79" {
			mode = "release"
		}

		if mode == "unsafe" && strings.HasPrefix(key.Description, "mkosi of ") {
			mode = "dev"
		}

		slog.Info("Platform keyring entry", "name", key.Description, "key", key.Fingerprint)
	}

	slog.Info("Starting up", "mode", mode, "release", s.RunningRelease)

	// If there's no network configuration in the state, attempt to fetch from the seed info.
	if s.System.Network == nil {
		s.System.Network, err = seed.GetNetwork(ctx, seed.SeedPartitionPath)
		if err != nil && !seed.IsMissing(err) {
			return err
		}
	}

	// Perform network configuration.
	slog.Info("Bringing up the network")
	err = systemd.ApplyNetworkConfiguration(ctx, s.System.Network, 10*time.Second)
	if err != nil {
		return err
	}

	// Get the provider.
	var provider string

	switch mode {
	case "release":
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
		// Run update function if we have a working provider.
		err = update(ctx, s, t, p)
		if err != nil {
			return err
		}
	}

	// Ensure  the "local" ZFS pool is available.
	slog.Info("Bringing up the local ZFS pool")
	err = zfs.ImportOrCreateLocalPool(ctx)
	if err != nil {
		return err
	}

	// Apply the system users.
	slog.Info("Refreshing users")
	err = systemd.RefreshUsers(ctx)
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

	return nil
}

func update(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider) error {
	// Determine what to install.
	toInstall := []string{"incus"}

	if len(s.Applications) == 0 {
		// Assume first start.
		apps, err := seed.GetApplications(ctx, seed.SeedPartitionPath)
		if err != nil && !seed.IsMissing(err) {
			return err
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
	update, err := p.GetOSUpdate(ctx)
	if err != nil {
		return err
	}

	// Apply the update.
	if update.Version() != s.RunningRelease {
		// Download the update into place.
		slog.Info("Downloading OS update", "release", update.Version())
		t.DisplayModal("Incus OS Update", "Downloading Incus OS update version "+update.Version(), 0, 0)
		err := update.Download(ctx, systemd.SystemUpdatesPath)
		if err != nil {
			return err
		}

		// Apply the update and reboot.
		slog.Info("Applying OS update", "release", update.Version())
		t.DisplayModal("Incus OS Update", "Applying Incus OS update version "+update.Version(), 0, 0)
		err = systemd.ApplySystemUpdate(ctx, update.Version(), true)
		if err != nil {
			return err
		}

		return nil
	}

	slog.Info("System is already running latest OS release", "release", s.RunningRelease)

	// Check for application updates.
	for _, appName := range toInstall {
		// Get the application.
		app, err := p.GetApplication(ctx, appName)
		if err != nil {
			return err
		}

		// Check if already up to date.
		if s.Applications[app.Name()].Version == app.Version() {
			slog.Info("System is already running latest application release", "application", app.Name(), "release", app.Version())

			continue
		}

		// Download the application.
		slog.Info("Downloading system extension", "application", app.Name(), "release", app.Version())
		t.DisplayModal("Incus OS Extension Update", "Downloading system extension "+app.Name()+" update "+update.Version(), 0, 0)
		err = app.Download(ctx, systemd.SystemExtensionsPath)
		if err != nil {
			return err
		}

		t.RemoveModal()

		// Record newly installed application.
		s.Applications[app.Name()] = state.Application{Version: app.Version()}
	}

	// Apply the system extensions.
	slog.Info("Refreshing system extensions")
	err = systemd.RefreshExtensions(ctx)
	if err != nil {
		return err
	}

	return nil
}
