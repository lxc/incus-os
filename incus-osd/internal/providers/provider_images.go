package providers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/osarch"
	"github.com/lxc/incus/v6/shared/subprocess"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// The images provider.
type images struct {
	state *state.State

	serverURL string
	updateCA  string

	releaseLastCheck time.Time
	releaseVersion   string
	releaseAssets    []string
}

func (p *images) ClearCache(_ context.Context) error {
	// Reset the last check time.
	p.releaseLastCheck = time.Time{}

	return nil
}

func (*images) RefreshRegister(_ context.Context) error {
	// No registration with the images provider.
	return ErrRegistrationUnsupported
}

func (*images) Register(_ context.Context, _ bool) error {
	// No registration with the images provider.
	return ErrRegistrationUnsupported
}

func (*images) Deregister(_ context.Context) error {
	// Since we can't register, deregister is a no-op.
	return nil
}

func (*images) Type() string {
	return "images"
}

func (*images) GetSecureBootCertUpdate(ctx context.Context, _ string) (SecureBootCertUpdate, error) {
	// Hardcode a single update for now until we have support for it in the provider.
	updateURL := "https://images.linuxcontainers.org/os/keys/efi/IncusOS_2026_R1.tar.gz"

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, updateURL, nil)
	if err != nil {
		return nil, ErrNoUpdateAvailable
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, ErrNoUpdateAvailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrNoUpdateAvailable
	}

	update := imagesSecureBootCertUpdate{
		url:     updateURL,
		version: "202601010000",
	}

	return &update, nil
}

func (p *images) GetOSUpdate(ctx context.Context, osName string) (OSUpdate, error) {
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

		if strings.HasPrefix(fileName, osName+"_") && strings.Contains(fileName, p.releaseVersion) {
			foundUpdateFile = true

			break
		}
	}

	if !foundUpdateFile {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the OS update struct.
	update := imagesOSUpdate{
		provider: p,
		assets:   p.releaseAssets,
		version:  p.releaseVersion,
	}

	return &update, nil
}

func (p *images) GetApplication(ctx context.Context, name string) (Application, error) {
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
	app := imagesApplication{
		provider: p,
		name:     name,
		assets:   p.releaseAssets,
		version:  p.releaseVersion,
	}

	return &app, nil
}

func (p *images) load(_ context.Context) error {
	// Set up the configuration.
	p.serverURL = p.state.System.Provider.Config.Config["server_url"]
	p.updateCA = p.state.System.Provider.Config.Config["update_ca"]

	// Basic validation.
	if p.serverURL == "" {
		p.serverURL = "https://images.linuxcontainers.org/os"
		p.updateCA = LXCUpdateCA
	}

	return nil
}

func (p *images) checkRelease(ctx context.Context) error {
	// Get local architecture.
	archName, err := osarch.ArchitectureGetLocal()
	if err != nil {
		return err
	}

	// Only talk to image server once an hour.
	if !p.releaseLastCheck.IsZero() && p.releaseLastCheck.Add(time.Hour).After(time.Now()) {
		return nil
	}

	// Get the latest signed index.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.serverURL+"/index.sjson", nil)
	if err != nil {
		return err
	}

	resp, err := p.tryRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("server failed to return expected file")
	}

	// Write the CA certificate.
	rootCA, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(rootCA, "%s", p.updateCA)
	if err != nil {
		return err
	}

	defer func() { _ = os.Remove(rootCA.Name()) }()

	// Validate signed index.
	verified := bytes.NewBuffer(nil)

	err = subprocess.RunCommandWithFds(ctx, resp.Body, verified, "openssl", "smime", "-verify", "-text", "-CAfile", rootCA.Name())
	if err != nil {
		return err
	}

	// Parse the update list.
	index := &apiupdate.Index{}

	err = json.NewDecoder(bytes.NewReader(verified.Bytes())).Decode(index)
	if err != nil {
		return err
	}

	if len(index.Updates) == 0 {
		return errors.New("no update available")
	}

	// Get the latest update.
	latestUpdate := index.Updates[0]

	if len(latestUpdate.Files) == 0 {
		return errors.New("no files in update")
	}

	latestUpdateFiles := make([]string, 0, len(latestUpdate.Files))
	for _, file := range latestUpdate.Files {
		if string(file.Architecture) != archName {
			continue
		}

		latestUpdateFiles = append(latestUpdateFiles, p.serverURL+"/"+latestUpdate.URL+"/"+file.Filename)
	}

	// Record the release.
	p.releaseLastCheck = time.Now()
	p.releaseVersion = latestUpdate.Version
	p.releaseAssets = latestUpdateFiles

	return nil
}

func (*images) tryRequest(req *http.Request) (*http.Response, error) {
	var err error

	for range 5 {
		var resp *http.Response

		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			return resp, nil
		}

		time.Sleep(time.Second)
	}

	return nil, err
}

func (*images) downloadAsset(ctx context.Context, assetURL string, target string, progressFunc func(float64)) error {
	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return err
	}

	// Get a reader for the release asset.
	resp, err := http.DefaultClient.Do(req)
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
		if progressFunc != nil && count%6 == 0 {
			progressFunc(float64(count*4*1024*1024) / srcSize)
		}

		count++
	}

	return nil
}

// An application from the images provider.
type imagesApplication struct {
	provider *images

	assets  []string
	name    string
	version string
}

func (a *imagesApplication) Name() string {
	return a.name
}

func (a *imagesApplication) Version() string {
	return a.version
}

func (a *imagesApplication) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(a.version, otherVersion)
}

func (a *imagesApplication) Download(ctx context.Context, target string, progressFunc func(float64)) error {
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

// An update from the images provider.
type imagesOSUpdate struct {
	provider *images

	assets  []string
	version string
}

func (o *imagesOSUpdate) Version() string {
	return o.version
}

func (o *imagesOSUpdate) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(o.version, otherVersion)
}

func (o *imagesOSUpdate) DownloadUpdate(ctx context.Context, osName string, targetPath string, progressFunc func(float64)) error {
	// Clear the target path.
	err := os.RemoveAll(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Create the target path.
	err = os.MkdirAll(targetPath, 0o700)
	if err != nil {
		return err
	}

	for _, asset := range o.assets {
		fileName := filepath.Base(asset)

		// Only select OS files.
		if !strings.HasPrefix(fileName, osName+"_") {
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
		err = o.provider.downloadAsset(ctx, asset, filepath.Join(targetPath, strings.TrimSuffix(fileName, ".gz")), progressFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *imagesOSUpdate) DownloadImage(ctx context.Context, imageType string, osName string, targetPath string, progressFunc func(float64)) (string, error) {
	// Create the target path.
	err := os.MkdirAll(targetPath, 0o700)
	if err != nil {
		return "", err
	}

	for _, asset := range o.assets {
		fileName := filepath.Base(asset)

		// Only select OS files.
		if !strings.HasPrefix(fileName, osName+"_") {
			continue
		}

		// Parse the file names.
		fields := strings.SplitN(fileName, ".", 2)
		if len(fields) != 2 {
			continue
		}

		// Continue if not the full image we're looking for.
		if fields[1] != imageType+".gz" {
			continue
		}

		// Download the image.
		err = o.provider.downloadAsset(ctx, asset, filepath.Join(targetPath, strings.TrimSuffix(fileName, ".gz")), progressFunc)

		return strings.TrimSuffix(fileName, ".gz"), err
	}

	return "", fmt.Errorf("failed to download image type '%s' for %s release %s", imageType, osName, o.version)
}

// Secure Boot key updates from the GitHub provider.
type imagesSecureBootCertUpdate struct {
	url     string
	version string
}

func (o *imagesSecureBootCertUpdate) Version() string {
	return o.version
}

func (o *imagesSecureBootCertUpdate) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(o.version, otherVersion)
}

func (o *imagesSecureBootCertUpdate) Download(ctx context.Context, osName string, target string) error {
	// #nosec G304
	f, err := os.Create(filepath.Join(target, osName+"_SecureBootKeys_"+o.version+".tar.gz"))
	if err != nil {
		return err
	}
	defer f.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("error downloading update: " + resp.Status)
	}

	_, err = io.Copy(f, resp.Body)

	return err
}
