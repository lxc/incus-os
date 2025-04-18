package providers

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The Local provider.
type local struct {
	config map[string]string
	path   string

	releaseAssets  []string
	releaseVersion string
}

func (p *local) GetOSUpdate(_ context.Context) (OSUpdate, error) {
	// Prepare the OS update struct.
	update := localOSUpdate{
		provider: p,
		assets:   p.releaseAssets,
		version:  p.releaseVersion,
	}

	return &update, nil
}

func (p *local) GetApplication(_ context.Context, name string) (Application, error) {
	// Prepare the application struct.
	app := localApplication{
		provider: p,
		name:     name,
		assets:   p.releaseAssets,
		version:  p.releaseVersion,
	}

	return &app, nil
}

func (p *local) load(ctx context.Context) error {
	// Use a hardcoded path for now.
	p.path = "/root/updates/"

	// Deal with missing path.
	_, err := os.Lstat(p.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrProviderUnavailable
		}

		return err
	}

	// Get latest release.
	err = p.checkRelease(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (p *local) checkRelease(_ context.Context) error {
	// Parse the version string.
	body, err := os.ReadFile(filepath.Join(p.path, "RELEASE"))
	if err != nil {
		return err
	}

	p.releaseVersion = strings.TrimSpace(string(body))

	// Build asset list.
	assets := []string{}

	entries, err := os.ReadDir(p.path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		assets = append(assets, filepath.Join(p.path, entry.Name()))
	}

	p.releaseAssets = assets

	return nil
}

func (p *local) copyAsset(_ context.Context, name string, target string) error {
	// Open the source.
	// #nosec G304
	src, err := os.Open(filepath.Join(p.path, name))
	if err != nil {
		return err
	}

	defer src.Close()

	// Open the destination.
	// #nosec G304
	dst, err := os.Create(filepath.Join(target, name))
	if err != nil {
		return err
	}

	defer dst.Close()

	// Copy the content.
	for {
		_, err := io.CopyN(dst, src, 4*1024*1024)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}
	}

	return nil
}

// An application from the Local provider.
type localApplication struct {
	provider *local

	assets  []string
	name    string
	version string
}

func (a *localApplication) Name() string {
	return a.name
}

func (a *localApplication) Version() string {
	return a.version
}

func (a *localApplication) Download(ctx context.Context, target string) error {
	// Create the target path.
	err := os.MkdirAll(target, 0o700)
	if err != nil {
		return err
	}

	for _, asset := range a.assets {
		appName := strings.TrimSuffix(filepath.Base(asset), ".raw")

		// Only select the desired applications.
		if appName != a.name {
			continue
		}

		// Copy the application.
		err = a.provider.copyAsset(ctx, filepath.Base(asset), target)
		if err != nil {
			return err
		}
	}

	return nil
}

// An update from the Local provider.
type localOSUpdate struct {
	provider *local

	assets  []string
	version string
}

func (o *localOSUpdate) Version() string {
	return o.version
}

func (o *localOSUpdate) Download(ctx context.Context, target string) error {
	// Clear the path.
	err := os.RemoveAll(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Create the target path.
	err = os.MkdirAll(target, 0o700)
	if err != nil {
		return err
	}

	for _, asset := range o.assets {
		// Only select OS files.
		if !strings.HasPrefix(filepath.Base(asset), "IncusOS_") {
			continue
		}

		// Parse the file names.
		fields := strings.SplitN(filepath.Base(asset), ".", 2)
		if len(fields) != 2 {
			continue
		}

		// Skip the full image.
		if fields[1] == "raw" {
			continue
		}

		// Download the actual update.
		err = o.provider.copyAsset(ctx, filepath.Base(asset), target)
		if err != nil {
			return err
		}
	}

	return nil
}
