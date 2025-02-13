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

	// Get current release.
	slog.Info("Getting local OS information")
	release, err := systemd.GetCurrentRelease(ctx)
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

	slog.Info("Starting up", "mode", mode, "app", "incus", "release", release)

	// Fetch the Github release.
	gh := github.NewClient(nil)

	ghRelease, _, err := gh.Repositories.GetLatestRelease(ctx, ghOrganization, ghRepository)
	if err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("Found latest %s/%s release", ghOrganization, ghRepository), "tag", ghRelease.GetTagName())

	assets, _, err := gh.Repositories.ListReleaseAssets(ctx, ghOrganization, ghRepository, ghRelease.GetID(), nil)
	if err != nil {
		return err
	}

	// Download OS updates.
	err = os.MkdirAll(systemd.SystemUpdatesPath, 0o700)
	if err != nil {
		return err
	}

	if release != ghRelease.GetName() {
		for _, asset := range assets {
			// Skip system extensions.
			if !strings.HasPrefix(asset.GetName(), "IncusOS_") {
				continue
			}

			fields := strings.SplitN(asset.GetName(), ".", 2)
			if len(fields) != 2 {
				continue
			}

			// Skip the full image.
			if fields[1] == "raw.gz" {
				continue
			}

			slog.Info("Downloading OS update", "file", asset.GetName(), "url", asset.GetBrowserDownloadURL())

			rc, _, err := gh.Repositories.DownloadReleaseAsset(ctx, ghOrganization, ghRepository, asset.GetID(), http.DefaultClient)
			if err != nil {
				return err
			}

			defer rc.Close()

			body, err := gzip.NewReader(rc)
			if err != nil {
				return err
			}

			defer body.Close()

			fd, err := os.Create(filepath.Join(systemd.SystemUpdatesPath, strings.TrimSuffix(asset.GetName(), ".gz")))
			if err != nil {
				return err
			}

			defer fd.Close()

			for {
				_, err = io.CopyN(fd, body, 4*1024*1024)
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					return err
				}
			}
		}

		err = systemd.ApplySystemUpdate(ctx, ghRelease.GetName(), true)
		if err != nil {
			return err
		}

		return nil
	}

	// Download system extensions.
	err = os.MkdirAll(systemd.SystemExtensionsPath, 0o700)
	if err != nil {
		return err
	}

	for _, asset := range assets {
		if !slices.Contains(incusExtensions, asset.GetName()) {
			continue
		}

		appName := strings.TrimSuffix(asset.GetName(), ".raw.gz")

		// Check if already up to date.
		if s.Applications[appName].Version == ghRelease.GetName() {
			continue
		}

		slog.Info("Downloading system extension", "application", appName, "file", asset.GetName(), "url", asset.GetBrowserDownloadURL())

		rc, _, err := gh.Repositories.DownloadReleaseAsset(ctx, ghOrganization, ghRepository, asset.GetID(), http.DefaultClient)
		if err != nil {
			return err
		}

		defer rc.Close()

		body, err := gzip.NewReader(rc)
		if err != nil {
			return err
		}

		defer body.Close()

		fd, err := os.Create(filepath.Join(systemd.SystemExtensionsPath, strings.TrimSuffix(asset.GetName(), ".gz")))
		if err != nil {
			return err
		}

		defer fd.Close()

		for {
			_, err = io.CopyN(fd, body, 4*1024*1024)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return err
			}
		}

		// Record newly installed application.
		s.Applications[appName] = state.Application{Version: ghRelease.GetName()}
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
