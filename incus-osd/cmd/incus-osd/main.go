// Package main is used for the incus-osd daemon.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/install"
	"github.com/lxc/incus-os/incus-osd/internal/keyring"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

var (
	varPath = "/var/lib/incus-os/"
	runPath = "/run/incus-os/"
)

func main() {
	// Check privileges.
	if os.Getuid() != 0 {
		_, _ = fmt.Fprintf(os.Stderr, "incus-osd must be run as root")
		os.Exit(1)
	}

	// Prepare a logger.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Run the daemon.
	err = run()
	if err != nil {
		slog.Error(err.Error())

		// Sleep for a second to allow output buffers to flush.
		time.Sleep(1 * time.Second)

		os.Exit(1)
	}
}

func run() error {
	ctx := context.TODO()

	// Check if we should try to install to a local disk.
	if install.IsInstallNeeded() {
		inst, err := install.NewInstall()
		if err != nil {
			return err
		}

		return inst.DoInstall(ctx)
	}

	// Create runtime path if missing.
	err := os.Mkdir(runPath, 0o700)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Setup listener.
	listenerPath := filepath.Join(runPath, "unix.socket")
	_ = os.Remove(listenerPath)

	listener, err := net.Listen("unix", listenerPath)
	if err != nil {
		return err
	}

	// Run startup tasks.
	err = startup(ctx)
	if err != nil {
		return err
	}

	// Setup server.
	server := &http.Server{
		Handler: http.NotFoundHandler(),

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return server.Serve(listener)
}

func startup(ctx context.Context) error {
	// Create storage path if missing.
	err := os.Mkdir(varPath, 0o700)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Get persistent state.
	s, err := state.LoadOrCreate(ctx, filepath.Join(varPath, "state.json"))
	if err != nil {
		return err
	}

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

	// Perform network configuration.
	network, err := seed.GetNetwork(ctx, seed.SeedPartitionPath)
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	err = systemd.ApplyNetworkConfiguration(ctx, network)
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
		err = update(ctx, s, p)
		if err != nil {
			return err
		}
	}

	// Apply the system users.
	slog.Info("Refreshing users")
	err = systemd.RefreshUsers(ctx)
	if err != nil {
		return err
	}

	// Run startup actions.
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

func update(ctx context.Context, s *state.State, p providers.Provider) error {
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
		err := update.Download(ctx, systemd.SystemUpdatesPath)
		if err != nil {
			return err
		}

		// Apply the update and reboot.
		slog.Info("Applying OS update", "release", update.Version())
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
		err = app.Download(ctx, systemd.SystemExtensionsPath)
		if err != nil {
			return err
		}

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
