package providers

import (
	"bytes"
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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/osarch"
	incustls "github.com/lxc/incus/v6/shared/tls"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// API structs.
type operationsCenterUpdate struct {
	Channels []string `json:"channels"`
	UUID     string   `json:"uuid"`
	Version  string   `json:"version"`

	Files []operationsCenterUpdateFile
}

type operationsCenterUpdateFile struct {
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	Component    string `json:"component"`
	Type         string `json:"type"`
	Architecture string `json:"architecture"`
	Sha256       string `json:"sha256"`

	url string
}

// The Operations Center provider.
type operationsCenter struct {
	state *state.State

	client *http.Client

	serverCertificate string
	serverURL         string
	serverToken       string

	lastCheck    time.Time
	latestUpdate *operationsCenterUpdate
	releaseMu    sync.Mutex
}

func (p *operationsCenter) ClearCache(_ context.Context) error {
	// Reset the last check time.
	p.lastCheck = time.Time{}

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

func (p *operationsCenter) Register(ctx context.Context, _ bool) error {
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
	resp, err := p.apiRequest(ctx, http.MethodPost, "/1.0/provisioning/servers?token="+p.state.System.Provider.Config.Config["server_token"], bytes.NewReader(data))
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

func (*operationsCenter) Deregister(_ context.Context) error {
	// At the moment, deregistration is not supported for Operations Center.
	return ErrDeregistrationUnsupported
}

func (*operationsCenter) Type() string {
	return "operations-center"
}

func (*operationsCenter) GetSecureBootCertUpdate(_ context.Context) (SecureBootCertUpdate, error) {
	// Eventually we'll have an API from OperationsCenter to query for any updates.
	return nil, ErrNoUpdateAvailable
}

func (p *operationsCenter) GetOSUpdate(ctx context.Context) (OSUpdate, error) {
	// Get latest release.
	latestUpdate, err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Check that an OS update is included.
	found := false

	for _, file := range latestUpdate.Files {
		if file.Component == string(apiupdate.UpdateFileComponentOS) {
			found = true

			break
		}
	}

	if !found {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the OS update struct.
	update := operationsCenterOSUpdate{
		provider:     p,
		latestUpdate: latestUpdate,
	}

	return &update, nil
}

func (p *operationsCenter) GetApplication(ctx context.Context, name string) (Application, error) {
	// Get latest release.
	latestUpdate, err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Check that an application update is included.
	found := false

	for _, file := range latestUpdate.Files {
		if file.Component == name {
			found = true

			break
		}
	}

	if !found {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the application struct.
	app := operationsCenterApplication{
		provider:     p,
		name:         name,
		latestUpdate: p.latestUpdate,
	}

	return &app, nil
}

func (p *operationsCenter) load(ctx context.Context) error {
	p.client = &http.Client{}

	// Set up the configuration.
	p.serverCertificate = p.state.System.Provider.Config.Config["server_certificate"]
	p.serverURL = p.state.System.Provider.Config.Config["server_url"]
	p.serverToken = p.state.System.Provider.Config.Config["server_token"]

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
		if errors.Is(err, applications.ErrNoPrimary) {
			// Don't try to setup the TLS client certificate if no primary application installed yet.
			return nil
		}

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
		if apiResp.Type == "error" {
			return nil, fmt.Errorf("error from operations center: %s", apiResp.Error)
		}

		return nil, errors.New("bad response from operations center")
	}

	return apiResp, nil
}

func (p *operationsCenter) checkRelease(ctx context.Context) (*operationsCenterUpdate, error) {
	// Acquire lock.
	p.releaseMu.Lock()
	defer p.releaseMu.Unlock()

	// Get local architecture.
	archName, err := osarch.ArchitectureGetLocal()
	if err != nil {
		return nil, err
	}

	// Only talk to Operations Center once an hour.
	if p.latestUpdate != nil && !p.lastCheck.IsZero() && p.lastCheck.Add(time.Hour).After(time.Now()) {
		return p.latestUpdate, nil
	}

	// API structs.
	// Get the latest release.
	apiResp, err := p.apiRequest(ctx, http.MethodGet, "/1.0/provisioning/updates?recursion=1", nil)
	if err != nil {
		return nil, err
	}

	// Parse the update list.
	updates := []operationsCenterUpdate{}

	err = apiResp.MetadataAsStruct(&updates)
	if err != nil {
		return nil, err
	}

	if len(updates) == 0 {
		return nil, ErrNoUpdateAvailable
	}

	// Get the latest update for the expected channel.
	var latestUpdate *operationsCenterUpdate

	for _, update := range updates {
		// Skip any update targeting the wrong channel(s).
		if update.Version != p.state.OS.RunningRelease && p.state.System.Update.Config.Channel != "" && !slices.Contains(update.Channels, p.state.System.Update.Config.Channel) {
			continue
		}

		latestUpdate = &update

		break
	}

	if latestUpdate == nil {
		return nil, ErrNoUpdateAvailable
	}

	// Get the file list.
	apiResp, err = p.apiRequest(ctx, http.MethodGet, "/1.0/provisioning/updates/"+latestUpdate.UUID+"/files", nil)
	if err != nil {
		return nil, err
	}

	// Parse the file list.
	files := []operationsCenterUpdateFile{}

	err = apiResp.MetadataAsStruct(&files)
	if err != nil {
		return nil, err
	}

	latestUpdateFiles := []operationsCenterUpdateFile{}

	for _, file := range files {
		if file.Architecture != "" && file.Architecture != archName {
			continue
		}

		file.url = p.serverURL + "/1.0/provisioning/updates/" + updates[0].UUID + "/files/" + file.Filename
		latestUpdateFiles = append(latestUpdateFiles, file)
	}

	latestUpdate.Files = latestUpdateFiles

	if len(latestUpdate.Files) == 0 {
		return nil, ErrNoUpdateAvailable
	}

	// Record the release.
	p.lastCheck = time.Now()
	p.latestUpdate = latestUpdate

	return latestUpdate, nil
}

// An application from the Operations Center provider.
type operationsCenterApplication struct {
	provider *operationsCenter

	name         string
	latestUpdate *operationsCenterUpdate
}

func (a *operationsCenterApplication) Name() string {
	return a.name
}

func (a *operationsCenterApplication) Version() string {
	return a.latestUpdate.Version
}

func (a *operationsCenterApplication) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(a.latestUpdate.Version, otherVersion)
}

func (a *operationsCenterApplication) Download(ctx context.Context, target string, progressFunc func(float64)) error {
	// Create the target path.
	err := os.MkdirAll(target, 0o700)
	if err != nil {
		return err
	}

	for _, file := range a.latestUpdate.Files {
		// Only select the desired applications.
		if file.Component != a.name {
			continue
		}

		fileURL := a.provider.serverURL + "/" + a.latestUpdate.Version + "/" + file.Filename
		targetName := strings.TrimSuffix(filepath.Base(file.Filename), ".gz")

		// Download the application.
		err = downloadAsset(ctx, http.DefaultClient, fileURL, file.Sha256, filepath.Join(target, targetName), progressFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

// An update from the Operations Center provider.
type operationsCenterOSUpdate struct {
	provider *operationsCenter

	latestUpdate *operationsCenterUpdate
}

func (o *operationsCenterOSUpdate) Version() string {
	return o.latestUpdate.Version
}

func (o *operationsCenterOSUpdate) IsNewerThan(otherVersion string) bool {
	return datetimeComparison(o.latestUpdate.Version, otherVersion)
}

func (o *operationsCenterOSUpdate) DownloadUpdate(ctx context.Context, target string, progressFunc func(float64)) error {
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

	for _, file := range o.latestUpdate.Files {
		// Only select OS updates.
		if file.Component != string(apiupdate.UpdateFileComponentOS) || !strings.HasPrefix(file.Type, "update-") {
			continue
		}

		fileURL := o.provider.serverURL + "/" + o.latestUpdate.Version + "/" + file.Filename
		targetName := strings.TrimSuffix(filepath.Base(file.Filename), ".gz")

		// Download the application.
		err = downloadAsset(ctx, http.DefaultClient, fileURL, file.Sha256, filepath.Join(target, targetName), progressFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *operationsCenterOSUpdate) DownloadImage(ctx context.Context, imageType string, target string, progressFunc func(float64)) (string, error) {
	// Create the target path.
	err := os.MkdirAll(target, 0o700)
	if err != nil {
		return "", err
	}

	for _, file := range o.latestUpdate.Files {
		// Only select OS updates.
		if file.Component != string(apiupdate.UpdateFileComponentOS) || file.Type != "image-"+imageType {
			continue
		}

		fileURL := o.provider.serverURL + "/" + o.latestUpdate.Version + "/" + file.Filename
		targetName := strings.TrimSuffix(filepath.Base(file.Filename), ".gz")

		// Download the application.
		err = downloadAsset(ctx, http.DefaultClient, fileURL, file.Sha256, filepath.Join(target, targetName), progressFunc)

		return targetName, err
	}

	return "", fmt.Errorf("failed to download image type '%s' for release %s", imageType, o.latestUpdate.Version)
}
