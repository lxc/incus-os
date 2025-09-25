package providers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/osarch"
	incustls "github.com/lxc/incus/v6/shared/tls"

	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// The Operations Center provider.
type operationsCenter struct {
	config map[string]string
	state  *state.State

	client *http.Client

	serverCertificate string
	serverURL         string
	serverToken       string

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

func (p *operationsCenter) RefreshRegister(ctx context.Context) error {
	// Check if registered.
	if !p.state.System.Provider.State.Registered {
		return nil
	}

	// API structs.
	type serverPut struct {
		ConnectionURL string `json:"connection_url"`
	}

	// Get the management address.
	mgmtAddr := p.state.ManagementAddress()
	if mgmtAddr == nil {
		return ErrRegistrationUnsupported
	}

	// Prepare the registration request.
	req := serverPut{
		ConnectionURL: "https://" + net.JoinHostPort(mgmtAddr.String(), "8443"),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	// Register.
	_, err = p.apiRequest(ctx, http.MethodPut, "/1.0/provisioning/servers/:self", bytes.NewReader(data))
	if err != nil {
		return err
	}

	return nil
}

func (p *operationsCenter) Register(ctx context.Context) error {
	// API structs.
	type serverPost struct {
		Name          string `json:"name"`
		ConnectionURL string `json:"connection_url"`
	}

	type serverPostResp struct {
		Certificate string `json:"certificate"`
	}

	// Get the management address.
	mgmtAddr := p.state.ManagementAddress()
	if mgmtAddr == nil {
		return ErrRegistrationUnsupported
	}

	// Prepare the registration request.
	req := serverPost{
		Name:          p.state.Hostname(),
		ConnectionURL: "https://" + net.JoinHostPort(mgmtAddr.String(), "8443"),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	// Register.
	resp, err := p.apiRequest(ctx, http.MethodPost, "/1.0/provisioning/servers?token="+p.config["server_token"], bytes.NewReader(data))
	if err != nil {
		return err
	}

	// Parse the response.
	registrationResp := serverPostResp{}

	err = resp.MetadataAsStruct(&registrationResp)
	if err != nil {
		return err
	}

	// Get the primary application.
	app, err := applications.GetPrimary(ctx, p.state)
	if err != nil {
		return err
	}

	// Get the server certificate.
	err = app.AddTrustedCertificate(ctx, p.serverURL, registrationResp.Certificate)
	if err != nil {
		return err
	}

	return nil
}

func (*operationsCenter) Type() string {
	return "operations-center"
}

func (*operationsCenter) GetSecureBootCertUpdate(_ context.Context, _ string) (SecureBootCertUpdate, error) {
	// Eventually we'll have an API from OperationsCenter to query for any updates.
	return nil, ErrNoUpdateAvailable
}

func (p *operationsCenter) GetOSUpdate(ctx context.Context, osName string) (OSUpdate, error) {
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

func (p *operationsCenter) load(ctx context.Context) error {
	p.client = &http.Client{}

	// Set up the configuration.
	p.serverCertificate = p.config["server_certificate"]
	p.serverURL = p.config["server_url"]
	p.serverToken = p.config["server_token"]

	// Basic validation.
	if p.serverURL == "" {
		return errors.New("no operations center URL provided")
	}

	if p.serverToken == "" {
		return errors.New("no operations center token provided")
	}

	// Prepare the TLS config.
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	// Setup the server for self-signed certirficates.
	if p.serverCertificate != "" {
		// Parse the provided certificate.
		certBlock, _ := pem.Decode([]byte(p.serverCertificate))
		if certBlock == nil {
			return errors.New("invalid remote certificate")
		}

		serverCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return fmt.Errorf("invalid remote certificate: %w", err)
		}

		// Add the certificate to the TLS config.
		incustls.TLSConfigWithTrustedCert(tlsConfig, serverCert)
	}

	// Set the client certificate (if present).
	err := p.configureClientCertificate(ctx, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to set client certificate: %w", err)
	}

	// Disable the use of the system proxy.
	proxy := func(_ *http.Request) (*url.URL, error) {
		return nil, nil //nolint:nilnil
	}

	// Configure the HTTP client with our TLS config.
	p.client.Transport = &http.Transport{
		Proxy:           proxy,
		TLSClientConfig: tlsConfig,
	}

	return nil
}

func (p *operationsCenter) configureClientCertificate(ctx context.Context, tlsConfig *tls.Config) error {
	// Get the primary application.
	app, err := applications.GetPrimary(ctx, p.state)
	if err != nil {
		return err
	}

	// Get the server certificate.
	cert, err := app.GetCertificate()
	if err != nil {
		return err
	}

	tlsConfig.Certificates = []tls.Certificate{*cert}

	return nil
}

func (p *operationsCenter) apiRequest(ctx context.Context, method string, path string, data io.Reader) (*api.Response, error) {
	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, method, p.serverURL+path, data)
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
		return nil, errors.New("bad response from operations center")
	}

	return apiResp, nil
}

func (p *operationsCenter) checkRelease(ctx context.Context) error {
	// Acquire lock.
	p.releaseMu.Lock()
	defer p.releaseMu.Unlock()

	// Get local architecture.
	archName, err := osarch.ArchitectureGetLocal()
	if err != nil {
		return err
	}

	// Only talk to Operations Center once an hour.
	if !p.releaseLastCheck.IsZero() && p.releaseLastCheck.Add(time.Hour).After(time.Now()) {
		return nil
	}

	// API structs.
	type update struct {
		Channels []string `json:"channels"`
		UUID     string   `json:"uuid"`
		Version  string   `json:"version"`
	}

	type updateFile struct {
		Filename     string `json:"filename"`
		Size         int64  `json:"size"`
		Component    string `json:"component"`
		Type         string `json:"type"`
		Architecture string `json:"architecture"`
	}

	// Get the latest release.
	apiResp, err := p.apiRequest(ctx, http.MethodGet, "/1.0/provisioning/updates?recursion=1", nil)
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
	apiResp, err = p.apiRequest(ctx, http.MethodGet, "/1.0/provisioning/updates/"+updates[0].UUID+"/files", nil)
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
		if file.Architecture != archName {
			continue
		}

		latestReleaseFiles = append(latestReleaseFiles, p.serverURL+"/1.0/provisioning/updates/"+updates[0].UUID+"/files/"+file.Filename)
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
		if progressFunc != nil && count%6 == 0 {
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

func (o *operationsCenterOSUpdate) DownloadUpdate(ctx context.Context, osName string, targetPath string, progressFunc func(float64)) error {
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

func (o *operationsCenterOSUpdate) DownloadImage(ctx context.Context, imageType string, osName string, targetPath string, progressFunc func(float64)) (string, error) {
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
