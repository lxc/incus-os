package main

import (
	"compress/gzip"
	"context"
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
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

var (
	ghOrganization   = "lxc"
	ghRepository     = "incus-os"
	osExtensions     = []string{"debug.raw.gz", "incus.raw.gz"}
	osExtensionsPath = "/var/lib/extensions"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func run() error {
	ctx := context.TODO()

	// Prepare a logger.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

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

	slog.Info("Starting up", "mode", mode, "app", "incus")

	// Fetch the system extensions.
	gh := github.NewClient(nil)

	release, _, err := gh.Repositories.GetLatestRelease(ctx, ghOrganization, ghRepository)
	if err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("Found latest %s/%s release", ghOrganization, ghRepository), "tag", release.GetTagName())

	assets, _, err := gh.Repositories.ListReleaseAssets(ctx, ghOrganization, ghRepository, release.GetID(), nil)
	if err != nil {
		return err
	}

	err = os.MkdirAll(osExtensionsPath, 0700)
	if err != nil {
		return err
	}

	for _, asset := range assets {
		if !slices.Contains(osExtensions, asset.GetName()) {
			continue
		}

		slog.Info("Downloading OS extension", "file", asset.GetName(), "url", asset.GetBrowserDownloadURL())

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

		fd, err := os.Create(filepath.Join(osExtensionsPath, strings.TrimSuffix(asset.GetName(), ".gz")))
		if err != nil {
			return err
		}

		defer fd.Close()

		_, err = io.Copy(fd, body)
		if err != nil {
			return err
		}
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
