package providers

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	ghapi "github.com/google/go-github/v68/github"
)

// The Github provider.
type github struct {
	gh           *ghapi.Client
	organization string
	repository   string

	config map[string]string

	releaseLastCheck time.Time
	releaseVersion   string
	releaseAssets    []*ghapi.ReleaseAsset
	releaseMu        sync.Mutex
}

func (p *github) ClearCache(_ context.Context) error {
	// Reset the last check time.
	p.releaseLastCheck = time.Time{}

	return nil
}

func (p *github) GetOSUpdate(ctx context.Context) (OSUpdate, error) {
	// Get latest release.
	err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Verify the list of returned assets for the OS update contains at least
	// one file for the release version, otherwise we shouldn't report an OS update.
	foundUpdateFile := false
	for _, asset := range p.releaseAssets {
		if strings.HasPrefix(asset.GetName(), "IncusOS_") && strings.Contains(asset.GetName(), p.releaseVersion) {
			foundUpdateFile = true

			break
		}
	}

	if !foundUpdateFile {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the OS update struct.
	update := githubOSUpdate{
		provider: p,
		assets:   p.releaseAssets,
		version:  p.releaseVersion,
	}

	return &update, nil
}

func (p *github) GetApplication(ctx context.Context, name string) (Application, error) {
	// Get latest release.
	err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Verify the list of returned assets contains a "<name>.raw.gz" file, otherwise
	// we shouldn't return an application update.
	foundUpdateFile := false
	for _, asset := range p.releaseAssets {
		if asset.GetName() == name+".raw.gz" {
			foundUpdateFile = true

			break
		}
	}

	if !foundUpdateFile {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the application struct.
	app := githubApplication{
		provider: p,
		name:     name,
		assets:   p.releaseAssets,
		version:  p.releaseVersion,
	}

	return &app, nil
}

func (p *github) load(_ context.Context) error {
	// Setup the Github client.
	p.gh = ghapi.NewClient(nil)

	// Fixed configuration for now.
	p.organization = "lxc"
	p.repository = "incus-os"

	return nil
}

func (*github) checkLimit(err error) error {
	_, ok := err.(*ghapi.RateLimitError) //nolint:errorlint
	if ok {
		return ErrProviderUnavailable
	}

	return err
}

func (p *github) checkRelease(ctx context.Context) error {
	// Acquire lock.
	p.releaseMu.Lock()
	defer p.releaseMu.Unlock()

	// Only talk to Github once an hour.
	if !p.releaseLastCheck.IsZero() && p.releaseLastCheck.Add(time.Hour).After(time.Now()) {
		return nil
	}

	// Get the latest release.
	release, _, err := p.gh.Repositories.GetLatestRelease(ctx, p.organization, p.repository)
	if err != nil {
		return p.checkLimit(err)
	}

	// Get the list of files for the release.
	assets, _, err := p.gh.Repositories.ListReleaseAssets(ctx, p.organization, p.repository, release.GetID(), nil)
	if err != nil {
		return p.checkLimit(err)
	}

	// Record the release.
	p.releaseLastCheck = time.Now()
	p.releaseVersion = release.GetName()
	p.releaseAssets = assets

	return nil
}

func (p *github) downloadAsset(ctx context.Context, assetID int64, target string) error {
	// Get a reader for the release asset.
	rc, _, err := p.gh.Repositories.DownloadReleaseAsset(ctx, p.organization, p.repository, assetID, http.DefaultClient)
	if err != nil {
		return p.checkLimit(err)
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

// An application from the Github provider.
type githubApplication struct {
	provider *github

	assets  []*ghapi.ReleaseAsset
	name    string
	version string
}

func (a *githubApplication) Name() string {
	return a.name
}

func (a *githubApplication) Version() string {
	return a.version
}

func (a *githubApplication) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(a.version, otherVersion)
}

func (a *githubApplication) Download(ctx context.Context, target string) error {
	// Create the target path.
	err := os.MkdirAll(target, 0o700)
	if err != nil {
		return err
	}

	for _, asset := range a.assets {
		appName := strings.TrimSuffix(asset.GetName(), ".raw.gz")

		// Only select the desired applications.
		if appName != a.name {
			continue
		}

		// Download the application.
		err = a.provider.downloadAsset(ctx, asset.GetID(), filepath.Join(target, strings.TrimSuffix(asset.GetName(), ".gz")))
		if err != nil {
			return err
		}
	}

	return nil
}

// An update from the Github provider.
type githubOSUpdate struct {
	provider *github

	assets  []*ghapi.ReleaseAsset
	version string
}

func (o *githubOSUpdate) Version() string {
	return o.version
}

func (o *githubOSUpdate) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(o.version, otherVersion)
}

func (o *githubOSUpdate) Download(ctx context.Context, target string) error {
	// Clear the target path.
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
		if !strings.HasPrefix(asset.GetName(), "IncusOS_") {
			continue
		}

		// Parse the file names.
		fields := strings.SplitN(asset.GetName(), ".", 2)
		if len(fields) != 2 {
			continue
		}

		// Skip the full image.
		if fields[1] == "img.gz" || fields[1] == "iso.gz" {
			continue
		}

		// Download the actual update.
		err = o.provider.downloadAsset(ctx, asset.GetID(), filepath.Join(target, strings.TrimSuffix(asset.GetName(), ".gz")))
		if err != nil {
			return err
		}
	}

	return nil
}
