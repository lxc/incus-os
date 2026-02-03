package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/osarch"
	"github.com/lxc/incus/v6/shared/subprocess"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/auth"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

type imagesAuthenticatedTransport struct {
	machineID string
}

func (t *imagesAuthenticatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	authToken, err := auth.GenerateToken(context.Background(), t.machineID)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-IncusOS-Authentication", authToken)

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("bad HTTP transport")
	}

	return transport.Clone().RoundTrip(req)
}

// The images provider.
type images struct {
	state *state.State

	serverURL string
	updateCA  string
	token     string
	client    *http.Client

	lastCheck    time.Time // In system's timezone.
	latestUpdate *apiupdate.UpdateFull
}

func (p *images) ClearCache(_ context.Context) error {
	// Reset the last check time.
	p.lastCheck = time.Time{}

	return nil
}

func (p *images) RefreshRegister(_ context.Context) error {
	if p.token != "" {
		return nil
	}

	return ErrRegistrationUnsupported
}

func (p *images) Register(ctx context.Context, _ bool) error {
	if p.token != "" {
		// Register our TPM public key with the image server.
		// This is then used to validate authentication headers.
		machineID, err := p.state.MachineID()
		if err != nil {
			return err
		}

		req, err := auth.GenerateRegistration(ctx, machineID, p.token)
		if err != nil {
			return err
		}

		reqBody, err := json.Marshal(req)
		if err != nil {
			return err
		}

		// Prepare the request.
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, p.serverURL+"/register", bytes.NewReader(reqBody))
		if err != nil {
			return errors.New("unable to create http request: " + err.Error())
		}

		// Get a reader for the release asset.
		resp, err := p.client.Do(r)
		if err != nil {
			return errors.New("unable to get http register response: " + err.Error())
		}

		defer resp.Body.Close()

		// Check the response.
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad HTTP response code for registration: %d", resp.StatusCode)
		}

		return nil
	}

	// No registration with the images provider.
	return ErrRegistrationUnsupported
}

func (*images) Deregister(_ context.Context) error {
	return nil
}

func (*images) Type() string {
	return "images"
}

func (p *images) GetSecureBootCertUpdate(ctx context.Context) (SecureBootCertUpdate, error) {
	// Get latest release.
	latestUpdate, err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Check if a SecureBoot update is included.
	found := false

	for _, file := range latestUpdate.Files {
		if file.Type == apiupdate.UpdateFileTypeUpdateSecureboot {
			found = true

			break
		}
	}

	if !found {
		return nil, ErrNoUpdateAvailable
	}

	update := imagesSecureBootCertUpdate{
		provider:     p,
		latestUpdate: latestUpdate,
	}

	return &update, nil
}

func (p *images) GetOSUpdate(ctx context.Context) (OSUpdate, error) {
	// Get latest release.
	latestUpdate, err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Check that an OS update is included.
	found := false

	for _, file := range latestUpdate.Files {
		if file.Component == apiupdate.UpdateFileComponentOS {
			found = true

			break
		}
	}

	if !found {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the OS update struct.
	update := imagesOSUpdate{
		provider:     p,
		latestUpdate: latestUpdate,
	}

	return &update, nil
}

func (p *images) GetApplicationUpdate(ctx context.Context, name string) (ApplicationUpdate, error) {
	// Get latest release.
	latestUpdate, err := p.checkRelease(ctx)
	if err != nil {
		return nil, err
	}

	// Check that an application update is included.
	found := false

	for _, file := range latestUpdate.Files {
		if string(file.Component) == name {
			found = true

			break
		}
	}

	if !found {
		return nil, ErrNoUpdateAvailable
	}

	// Prepare the application struct.
	app := imagesApplication{
		provider:     p,
		name:         name,
		latestUpdate: latestUpdate,
	}

	return &app, nil
}

func (p *images) load(_ context.Context) error {
	// Set up the configuration.
	p.serverURL = p.state.System.Provider.Config.Config["server_url"]
	p.updateCA = p.state.System.Provider.Config.Config["update_ca"]
	p.token = p.state.System.Provider.Config.Config["token"]
	p.client = http.DefaultClient

	// Basic validation.
	if p.serverURL == "" {
		var err error

		p.serverURL = "https://images.linuxcontainers.org/os"

		p.updateCA, err = GetUpdateCACert()
		if err != nil {
			return err
		}
	}

	// Authenticated clients.
	if p.token != "" {
		machineID, err := p.state.MachineID()
		if err != nil {
			return err
		}

		transport := &imagesAuthenticatedTransport{
			machineID: machineID,
		}

		p.client = &http.Client{
			Transport: transport,
		}
	}

	return nil
}

func (p *images) checkRelease(ctx context.Context) (*apiupdate.UpdateFull, error) {
	// Only talk to image server once an hour.
	if p.latestUpdate != nil && !p.lastCheck.IsZero() && p.lastCheck.Add(time.Hour).After(time.Now()) {
		return p.latestUpdate, nil
	}

	// Get local architecture.
	archName, err := osarch.ArchitectureGetLocal()
	if err != nil {
		return nil, err
	}

	// Get the latest signed index.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.serverURL+"/index.sjson", nil)
	if err != nil {
		return nil, err
	}

	resp, err := tryRequest(p.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("server failed to return expected file")
	}

	// Write the CA certificate.
	rootCA, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}

	_, err = rootCA.WriteString(p.updateCA)
	if err != nil {
		return nil, err
	}

	defer func() { _ = os.Remove(rootCA.Name()) }()

	// Validate signed index.
	verified := bytes.NewBuffer(nil)

	err = subprocess.RunCommandWithFds(ctx, resp.Body, verified, "openssl", "smime", "-verify", "-text", "-CAfile", rootCA.Name())
	if err != nil {
		return nil, err
	}

	// Parse the update list.
	index := &apiupdate.Index{}

	err = json.NewDecoder(bytes.NewReader(verified.Bytes())).Decode(index)
	if err != nil {
		return nil, err
	}

	// Get the latest update for the expected channel.
	var latestUpdate *apiupdate.UpdateFull

	for _, update := range index.Updates {
		// Skip any update targeting the wrong channel(s).
		if update.Version != p.state.OS.RunningRelease && p.state.System.Update.Config.Channel != "" && !slices.Contains(update.Channels, p.state.System.Update.Config.Channel) {
			continue
		}

		// Skip any update with no files.
		if len(update.Files) == 0 {
			continue
		}

		// Strip files for other architectures.
		newFiles := []apiupdate.UpdateFile{}

		for _, file := range update.Files {
			if file.Architecture != "" && string(file.Architecture) != archName {
				continue
			}

			newFiles = append(newFiles, file)
		}

		update.Files = newFiles

		// Skip images with no suitable files.
		if len(update.Files) == 0 {
			continue
		}

		latestUpdate = &update

		break
	}

	if latestUpdate == nil {
		return nil, ErrNoUpdateAvailable
	}

	// Record the release.
	p.lastCheck = time.Now()
	p.latestUpdate = latestUpdate

	return latestUpdate, nil
}

// An application from the images provider.
type imagesApplication struct {
	provider *images

	name         string
	latestUpdate *apiupdate.UpdateFull
}

func (a *imagesApplication) Name() string {
	return a.name
}

func (a *imagesApplication) Version() string {
	return a.latestUpdate.Version
}

func (a *imagesApplication) IsNewerThan(otherVersion string) bool {
	return DatetimeComparison(a.latestUpdate.Version, otherVersion)
}

func (a *imagesApplication) Download(ctx context.Context, targetPath string, progressFunc func(float64)) error {
	// Create the target path.
	err := os.MkdirAll(targetPath, 0o700)
	if err != nil {
		return err
	}

	for _, file := range a.latestUpdate.Files {
		// Only select the desired applications.
		if string(file.Component) != a.name {
			continue
		}

		fileURL := a.provider.serverURL + "/" + a.latestUpdate.Version + "/" + file.Filename
		targetName := strings.TrimSuffix(filepath.Base(file.Filename), ".gz")

		// Download the application.
		err = downloadAsset(ctx, a.provider.client, fileURL, file.Sha256, filepath.Join(targetPath, targetName), progressFunc)
		if err != nil {
			return fmt.Errorf("while downloading %s, got error '%s'", fileURL, err.Error())
		}
	}

	return nil
}

// An update from the images provider.
type imagesOSUpdate struct {
	provider *images

	latestUpdate *apiupdate.UpdateFull
}

func (o *imagesOSUpdate) Version() string {
	return o.latestUpdate.Version
}

func (o *imagesOSUpdate) IsNewerThan(otherVersion string) bool {
	return DatetimeComparison(o.latestUpdate.Version, otherVersion)
}

func (o *imagesOSUpdate) Download(ctx context.Context, targetPath string, progressFunc func(float64)) error {
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

	for _, file := range o.latestUpdate.Files {
		// Only select OS updates.
		if file.Component != apiupdate.UpdateFileComponentOS || !slices.Contains([]apiupdate.UpdateFileType{apiupdate.UpdateFileTypeUpdateEFI, apiupdate.UpdateFileTypeUpdateUsr, apiupdate.UpdateFileTypeUpdateUsrVerity, apiupdate.UpdateFileTypeUpdateUsrVeritySignature}, file.Type) {
			continue
		}

		fileURL := o.provider.serverURL + "/" + o.latestUpdate.Version + "/" + file.Filename
		targetName := strings.TrimSuffix(filepath.Base(file.Filename), ".gz")

		// Download the application.
		err = downloadAsset(ctx, o.provider.client, fileURL, file.Sha256, filepath.Join(targetPath, targetName), progressFunc)
		if err != nil {
			return fmt.Errorf("while downloading %s, got error '%s'", fileURL, err.Error())
		}
	}

	return nil
}

func (o *imagesOSUpdate) DownloadImage(ctx context.Context, imageType string, targetPath string, progressFunc func(float64)) (string, error) {
	// Create the target path.
	err := os.MkdirAll(targetPath, 0o700)
	if err != nil {
		return "", err
	}

	for _, file := range o.latestUpdate.Files {
		// Only select OS updates.
		if file.Component != apiupdate.UpdateFileComponentOS || string(file.Type) != "image-"+imageType {
			continue
		}

		fileURL := o.provider.serverURL + "/" + o.latestUpdate.Version + "/" + file.Filename
		targetName := strings.TrimSuffix(filepath.Base(file.Filename), ".gz")

		// Download the application.
		err = downloadAsset(ctx, o.provider.client, fileURL, file.Sha256, filepath.Join(targetPath, targetName), progressFunc)

		return targetName, err
	}

	return "", fmt.Errorf("failed to download image type '%s' for release %s", imageType, o.latestUpdate.Version)
}

// Secure Boot key updates from the images provider.
type imagesSecureBootCertUpdate struct {
	provider *images

	latestUpdate *apiupdate.UpdateFull
}

func (o *imagesSecureBootCertUpdate) Version() string {
	return o.latestUpdate.Version
}

func (o *imagesSecureBootCertUpdate) GetFilename() string {
	return "SecureBootKeys_" + o.latestUpdate.Version + ".tar"
}

func (o *imagesSecureBootCertUpdate) IsNewerThan(otherVersion string) bool {
	return DatetimeComparison(o.latestUpdate.Version, otherVersion)
}

func (o *imagesSecureBootCertUpdate) Download(ctx context.Context, targetPath string, _ func(float64)) error {
	// Create the target path.
	err := os.MkdirAll(targetPath, 0o700)
	if err != nil {
		return err
	}

	for _, file := range o.latestUpdate.Files {
		// Only select the SecureBoot update.
		if file.Type != apiupdate.UpdateFileTypeUpdateSecureboot {
			continue
		}

		fileURL := o.provider.serverURL + "/" + o.latestUpdate.Version + "/" + file.Filename

		// Download the application.
		err = downloadAsset(ctx, o.provider.client, fileURL, file.Sha256, filepath.Join(targetPath, o.GetFilename()), nil)
		if err != nil {
			return fmt.Errorf("while downloading %s, got error '%s'", fileURL, err.Error())
		}
	}

	return nil
}
