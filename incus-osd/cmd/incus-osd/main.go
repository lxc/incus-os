// Package main is used for the incus-osd daemon.
package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-github/v68/github"

	"github.com/lxc/incus-os/incus-osd/internal/keyring"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

var (
	varPath = "/var/lib/incus-os/"

	ghOrganization = "lxc"
	ghRepository   = "incus-os"

	incusExtensions = []string{"debug.raw.gz", "incus.raw.gz"}
)

func githubDownloadAsset(ctx context.Context, ghOrganization string, ghRepository string, assetID int64, target string) error {
	// Get a new Github client.
	gh := github.NewClient(nil)

	// Get a reader for the release asset.
	rc, _, err := gh.Repositories.DownloadReleaseAsset(ctx, ghOrganization, ghRepository, assetID, http.DefaultClient)
	if err != nil {
		return err
	}

	defer rc.Close()

	// Setup a gzip reader to decompress during streaming.
	body, err := gzip.NewReader(rc)
	if err != nil {
		return err
	}

	defer body.Close()

	// Create the target path.
	// #nosec G304
	fd, err := os.Create(target)
	if err != nil {
		return err
	}

	defer fd.Close()

	// Read from the decompressor in chunks to avoid excessive memory consumption.
	for {
		_, err = io.CopyN(fd, body, 4*1024*1024)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}
	}

	return nil
}

func updateOS(ctx context.Context, runningRelease string) error {
	// Get a new Github client.
	gh := github.NewClient(nil)

	// Get the latest release.
	ghRelease, _, err := gh.Repositories.GetLatestRelease(ctx, ghOrganization, ghRepository)
	if err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("Found latest %s/%s release", ghOrganization, ghRepository), "tag", ghRelease.GetTagName())

	// Get the list of files for the release.
	assets, _, err := gh.Repositories.ListReleaseAssets(ctx, ghOrganization, ghRepository, ghRelease.GetID(), nil)
	if err != nil {
		return err
	}

	// Create the target path.
	err = os.MkdirAll(systemd.SystemUpdatesPath, 0o700)
	if err != nil {
		return err
	}

	// Check if already up to date.
	if runningRelease == ghRelease.GetName() {
		slog.Info("System is already running latest OS release", "release", runningRelease)

		return nil
	}

	for _, asset := range assets {
		// Only select OS files.
		if !strings.HasPrefix(asset.GetName(), "IncusOS_") {
			continue
		}

		// Parse the file names.
		fields := strings.SplitN(asset.GetName(), ".", 2)
		if len(fields) != 2 {
			continue
		}

		// Skip the full image.
		if fields[1] == "raw.gz" {
			continue
		}

		// Download the actual update.
		slog.Info("Downloading OS update", "file", asset.GetName(), "url", asset.GetBrowserDownloadURL())
		err = githubDownloadAsset(ctx, ghOrganization, ghRepository, asset.GetID(), filepath.Join(systemd.SystemUpdatesPath, strings.TrimSuffix(asset.GetName(), ".gz")))
		if err != nil {
			return err
		}
	}

	// Apply the update and reboot.
	err = systemd.ApplySystemUpdate(ctx, ghRelease.GetName(), true)
	if err != nil {
		return err
	}

	return nil
}

func updateApplications(ctx context.Context, s *state.State) error {
	// Get a new Github client.
	gh := github.NewClient(nil)

	// Get the latest release.
	ghRelease, _, err := gh.Repositories.GetLatestRelease(ctx, ghOrganization, ghRepository)
	if err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("Found latest %s/%s release", ghOrganization, ghRepository), "tag", ghRelease.GetTagName())

	// Get the list of files for the release.
	assets, _, err := gh.Repositories.ListReleaseAssets(ctx, ghOrganization, ghRepository, ghRelease.GetID(), nil)
	if err != nil {
		return err
	}

	// Create the target path.
	err = os.MkdirAll(systemd.SystemExtensionsPath, 0o700)
	if err != nil {
		return err
	}

	for _, asset := range assets {
		// Only select the desired applications.
		if !slices.Contains(incusExtensions, asset.GetName()) {
			continue
		}

		appName := strings.TrimSuffix(asset.GetName(), ".raw.gz")

		// Check if already up to date.
		if s.Applications[appName].Version == ghRelease.GetName() {
			slog.Info("System is already running latest application release", "application", appName, "release", ghRelease.GetName())

			continue
		}

		// Download the application.
		slog.Info("Downloading system extension", "application", appName, "file", asset.GetName(), "url", asset.GetBrowserDownloadURL())
		err = githubDownloadAsset(ctx, ghOrganization, ghRepository, asset.GetID(), filepath.Join(systemd.SystemExtensionsPath, strings.TrimSuffix(asset.GetName(), ".gz")))
		if err != nil {
			return err
		}

		// Record newly installed application.
		s.Applications[appName] = state.Application{Version: ghRelease.GetName()}
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func run() error {
	ctx := context.TODO()

	// Prepare a logger.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

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

	defer func() { _ = s.Save(context.Background()) }()

	// Get running release.
	slog.Info("Getting local OS information")
	runningRelease, err := systemd.GetCurrentRelease(ctx)
	if err != nil {
		return err
	}

	// Check kernel keyring.
	slog.Info("Getting trusted system keys")
	keys, err := keyring.GetKeys(ctx, keyring.PlatformKeyring)
	if err != nil {
		return err
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

	slog.Info("Starting up", "mode", mode, "app", "incus", "release", runningRelease)

	// Check for OS updates.
	err = updateOS(ctx, runningRelease)
	if err != nil {
		return err
	}

	// Check for application updates.
	err = updateApplications(ctx, s)
	if err != nil {
		return err
	}

	// Apply the system extensions.
	slog.Info("Refreshing system extensions")
	err = systemd.RefreshExtensions(ctx)
	if err != nil {
		return err
	}

	// Apply the system users.
	slog.Info("Refreshing users")
	err = systemd.RefreshUsers(ctx)
	if err != nil {
		return err
	}

	// Enable and start Incus.
	slog.Info("Starting Incus")
	err = systemd.EnableUnit(ctx, true, "incus.socket", "incus-lxcfs.service", "incus-startup.service", "incus.service")
	if err != nil {
		return err
	}

	return nil
}
