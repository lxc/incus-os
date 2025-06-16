package providers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/osarch"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// The Operations Center provider.
type operationsCenter struct {
	config map[string]string
	state  *state.State

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

	// Connect to Incus.
	c, err := incusclient.ConnectIncusUnix("", nil)
	if err != nil {
		return err
	}

	// Add the certificate.
	cert := api.CertificatesPost{}
	cert.Name = p.serverURL
	cert.Type = "client"
	cert.Certificate = registrationResp.Certificate

	err = c.CreateCertificate(cert)
	if err != nil {
		return err
	}

	return nil
}

func (*operationsCenter) Type() string {
	return "operations-center"
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

func (p *operationsCenter) configureTLS() error {
	// Load the certificate.
	tlsClientCert, err := os.ReadFile("/var/lib/incus/server.crt")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	tlsClientKey, err := os.ReadFile("/var/lib/incus/server.key")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	// Create the TLS config.
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	clientCert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
	if err != nil {
		return err
	}

	tlsConfig.Certificates = []tls.Certificate{clientCert}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	p.client.Transport = tr

	return nil
}

func (p *operationsCenter) apiRequest(ctx context.Context, method string, path string, data io.Reader) (*api.Response, error) {
	// Attempt to configure TLS on the client if needed.
	if p.client.Transport == nil {
		err := p.configureTLS()
		if err != nil {
			return nil, err
		}
	}

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
		return nil, errors.New("bad response from update API")
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
		Channel string `json:"channel"`
		UUID    string `json:"uuid"`
		Version string `json:"version"`
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

func (o *operationsCenterOSUpdate) Download(ctx context.Context, osName string, target string, progressFunc func(float64)) error {
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
		err = o.provider.downloadAsset(ctx, asset, filepath.Join(target, strings.TrimSuffix(fileName, ".gz")), progressFunc)
		if err != nil {
			return err
		}
	}

	return nil
}
