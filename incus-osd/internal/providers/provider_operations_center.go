package providers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lxc/incus/v6/shared/api"
)

// The Operations Center provider.
type operationsCenter struct {
	config map[string]string

	client *http.Client

	serverURL   string
	serverToken string

	releaseLastCheck time.Time
	releaseVersion   string
	releaseAssets    []string
	releaseMu        sync.Mutex
}

func (p *operationsCenter) ClearCache(_ context.Context) error {
	// Reset the last check time.
	p.releaseLastCheck = time.Time{}

	return nil
}

func (*operationsCenter) Type() string {
	return "operations-center"
}

func (p *operationsCenter) GetOSUpdate(ctx context.Context) (OSUpdate, error) {
	// Get latest release.
	err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Verify the list of returned assets for the OS update contains at least
	// one file for the release version, otherwise we shouldn't report an OS update.
	foundUpdateFile := false
	for _, asset := range p.releaseAssets {
		fileName := filepath.Base(asset)

		if strings.HasPrefix(fileName, "IncusOS_") && strings.Contains(fileName, p.releaseVersion) {
			foundUpdateFile = true

			break
		}
	}

	if !foundUpdateFile {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the OS update struct.
	update := operationsCenterOSUpdate{
		provider: p,
		assets:   p.releaseAssets,
		version:  p.releaseVersion,
	}

	return &update, nil
}

func (p *operationsCenter) GetApplication(ctx context.Context, name string) (Application, error) {
	// Get latest release.
	err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Verify the list of returned assets contains a "<name>.raw.gz" file, otherwise
	// we shouldn't return an application update.
	foundUpdateFile := false
	for _, asset := range p.releaseAssets {
		fileName := filepath.Base(asset)

		if fileName == name+".raw.gz" {
			foundUpdateFile = true

			break
		}
	}

	if !foundUpdateFile {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the application struct.
	app := operationsCenterApplication{
		provider: p,
		name:     name,
		assets:   p.releaseAssets,
		version:  p.releaseVersion,
	}

	return &app, nil
}

func (p *operationsCenter) load(_ context.Context) error {
	p.client = &http.Client{}

	// Set up the configuration.
	p.serverURL = p.config["server_url"]
	p.serverToken = p.config["server_token"]

	// Basic validation.
	if p.serverURL == "" {
		return errors.New("no operations center URL provided")
	}

	if p.serverToken == "" {
		return errors.New("no operations center token provided")
	}

	return nil
}

func (p *operationsCenter) apiRequest(ctx context.Context, path string) (*api.Response, error) {
	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.serverURL+path, nil)
	if err != nil {
		return nil, err
	}

	// Make the REST call.
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Read the body.
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Convert to an Incus response struct.
	apiResp := &api.Response{}
	err = json.Unmarshal(content, apiResp)
	if err != nil {
		return nil, err
	}

	// Quick validation.
	if apiResp.Type != "sync" || apiResp.StatusCode != http.StatusOK {
		return nil, errors.New("bad response from update API")
	}

	return apiResp, nil
}

func (p *operationsCenter) checkRelease(ctx context.Context) error {
	// Acquire lock.
	p.releaseMu.Lock()
	defer p.releaseMu.Unlock()

	// Only talk to Operations Center once an hour.
	if !p.releaseLastCheck.IsZero() && p.releaseLastCheck.Add(time.Hour).After(time.Now()) {
		return nil
	}

	// API structs.
	type update struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	}

	type updateFile struct {
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}

	// Get the latest release.
	apiResp, err := p.apiRequest(ctx, "/1.0/provisioning/updates?recursion=1")
	if err != nil {
		return err
	}

	// Parse the update list.
	updates := []update{}
	err = apiResp.MetadataAsStruct(&updates)
	if err != nil {
		return err
	}

	if len(updates) == 0 {
		return errors.New("no update available")
	}

	// Get the latest release.
	latestRelease := updates[0].Version

	// Get the file list.
	apiResp, err = p.apiRequest(ctx, "/1.0/provisioning/updates/"+updates[0].ID+"/files")
	if err != nil {
		return err
	}

	// Parse the file list.
	files := []updateFile{}
	err = apiResp.MetadataAsStruct(&files)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return errors.New("no files in update")
	}

	latestReleaseFiles := make([]string, 0, len(files))
	for _, file := range files {
		latestReleaseFiles = append(latestReleaseFiles, p.serverURL+"/1.0/provisioning/updates/"+updates[0].ID+"/files/"+file.Filename)
	}

	// Record the release.
	p.releaseLastCheck = time.Now()
	p.releaseVersion = latestRelease
	p.releaseAssets = latestReleaseFiles

	return nil
}

func (p *operationsCenter) downloadAsset(ctx context.Context, assetURL string, target string, progressFunc func(float64)) error {
	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return err
	}

	// Get a reader for the release asset.
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Get the release asset size.
	srcSize := float64(resp.ContentLength)

	// Setup a gzip reader to decompress during streaming.
	body, err := gzip.NewReader(resp.Body)
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
	count := int64(0)
	for {
		_, err = io.CopyN(fd, body, 4*1024*1024)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		// Update progress every 24MiB.
		if count%6 == 0 {
			progressFunc(float64(count*4*1024*1024) / srcSize)
		}
		count++
	}

	return nil
}

// An application from the Operations Center provider.
type operationsCenterApplication struct {
	provider *operationsCenter

	assets  []string
	name    string
	version string
}

func (a *operationsCenterApplication) Name() string {
	return a.name
}

func (a *operationsCenterApplication) Version() string {
	return a.version
}

func (a *operationsCenterApplication) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(a.version, otherVersion)
}

func (a *operationsCenterApplication) Download(ctx context.Context, target string, progressFunc func(float64)) error {
	// Create the target path.
	err := os.MkdirAll(target, 0o700)
	if err != nil {
		return err
	}

	for _, asset := range a.assets {
		fileName := filepath.Base(asset)

		appName := strings.TrimSuffix(fileName, ".raw.gz")

		// Only select the desired applications.
		if appName != a.name {
			continue
		}

		// Download the application.
		err = a.provider.downloadAsset(ctx, asset, filepath.Join(target, strings.TrimSuffix(fileName, ".gz")), progressFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

// An update from the Operations Center provider.
type operationsCenterOSUpdate struct {
	provider *operationsCenter

	assets  []string
	version string
}

func (o *operationsCenterOSUpdate) Version() string {
	return o.version
}

func (o *operationsCenterOSUpdate) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(o.version, otherVersion)
}

func (o *operationsCenterOSUpdate) Download(ctx context.Context, target string, progressFunc func(float64)) error {
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
		fileName := filepath.Base(asset)

		// Only select OS files.
		if !strings.HasPrefix(fileName, "IncusOS_") {
			continue
		}

		// Parse the file names.
		fields := strings.SplitN(fileName, ".", 2)
		if len(fields) != 2 {
			continue
		}

		// Skip the full image.
		if fields[1] == "img.gz" || fields[1] == "iso.gz" {
			continue
		}

		// Download the actual update.
		err = o.provider.downloadAsset(ctx, asset, filepath.Join(target, strings.TrimSuffix(fileName, ".gz")), progressFunc)
		if err != nil {
			return err
		}
	}

	return nil
}
