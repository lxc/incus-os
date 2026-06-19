package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	ocapi "github.com/FuturFusion/operations-center/shared/api"
	"github.com/lxc/incus/v7/shared/osarch"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/auth"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

var imageRegisterMu sync.Mutex

type imagesAuthenticatedTransport struct {
	authParam bool
	machineID string
}

func (t *imagesAuthenticatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	authToken, err := auth.GenerateToken(context.Background(), t.machineID)
	if err != nil {
		return nil, err
	}

	if t.authParam {
		q := req.URL.Query()
		q.Set("token", authToken)
		req.URL.RawQuery = q.Encode()
	} else {
		req.Header.Set("X-IncusOS-Authentication", authToken)
	}

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
	authParam bool
	token     string
	client    *http.Client

	ignoreSignedJSON bool // If true, don't validate JSON metadata signature, but ONLY if the request is made via HTTPS.

	lastCheck    time.Time // In system's timezone.
	latestUpdate *apiupdate.UpdateFull
	releaseMu    sync.Mutex
}

func (p *images) ClearCache(_ context.Context) error {
	// Reset the last check time.
	p.lastCheck = time.Time{}

	return nil
}

func (p *images) RefreshRegister(_ context.Context, _ ocapi.ServerSelfUpdateCause) error {
	if p.token != "" {
		return nil
	}

	return ErrRegistrationUnsupported
}

func (p *images) Register(ctx context.Context) error {
	// Nothing to do if currently registered.
	if p.state.System.Provider.State.Registered {
		return nil
	}

	if p.token != "" { //nolint:nestif
		// Prevent concurent registration attempts.
		// The image provider triggers Register on load, so it's
		// possible/likely that we get two concurrent registration attempts during
		// first boot.
		imageRegisterMu.Lock()
		defer imageRegisterMu.Unlock()

		// Check that another goroutine didn't register the system.
		if p.state.System.Provider.State.Registered {
			return nil
		}

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
		var r *http.Request

		if p.authParam {
			u, err := url.Parse(p.serverURL + "/register")
			if err != nil {
				return err
			}

			token := base64.RawURLEncoding.EncodeToString(reqBody)
			u.Path = filepath.Join(u.Path, token)

			r, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
			if err != nil {
				return errors.New("unable to create http request: " + err.Error())
			}
		} else {
			r, err = http.NewRequestWithContext(ctx, http.MethodPost, p.serverURL+"/register", bytes.NewReader(reqBody))
			if err != nil {
				return errors.New("unable to create http request: " + err.Error())
			}
		}

		// Send the request.
		resp, err := p.client.Do(r)
		if err != nil {
			return errors.New("unable to get http register response: " + err.Error())
		}

		defer resp.Body.Close()

		// Check the response.
		if resp.StatusCode != http.StatusOK {
			switch resp.StatusCode {
			case http.StatusBadRequest:
				// The remote server should provide an error to return.
				b, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}

				return errors.New(string(b))
			default:
				return fmt.Errorf("bad HTTP response code for registration: %d", resp.StatusCode)
			}
		}

		// Log our successful registration and save state.
		slog.InfoContext(ctx, "Server successfully registered with the 'images' provider")

		p.state.System.Provider.State.Registered = true

		return p.state.Save()
	}

	// No registration with the images provider.
	return ErrRegistrationUnsupported
}

func (p *images) Deregister(ctx context.Context) error {
	// Log our successful deregistration and save state.
	slog.InfoContext(ctx, "Server successfully deregistered from the 'images' provider")

	p.state.System.Provider.State.Registered = false

	return p.state.Save()
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
		if filepath.Base(file.Filename) == name+".raw.gz" && file.Type == apiupdate.UpdateFileTypeApplication {
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

func (p *images) load(ctx context.Context) error {
	// Set up the configuration.
	p.serverURL = p.state.System.Provider.Config.Config["server_url"]
	p.updateCA = p.state.System.Provider.Config.Config["update_ca"]
	p.authParam = strings.ToLower(p.state.System.Provider.Config.Config["authentication_by_query_param"]) == "true"
	p.token = p.state.System.Provider.Config.Config["token"]
	p.client = &http.Client{}

	// Set default server URL if not configured.
	if p.serverURL == "" {
		p.serverURL = "https://images.linuxcontainers.org/os"
	}

	// Set default update CA if not configured.
	if p.updateCA == "" && !p.ignoreSignedJSON {
		var err error

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
			authParam: p.authParam,
		}

		p.client = &http.Client{
			Transport: transport,
		}

		// The images provider can register immediately, since no local application state
		// needs to be updated post-registration.
		err = p.Register(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *images) checkRelease(ctx context.Context) (*apiupdate.UpdateFull, error) {
	// Acquire lock.
	p.releaseMu.Lock()
	defer p.releaseMu.Unlock()

	// Only talk to image server once an hour.
	if p.latestUpdate != nil && !p.lastCheck.IsZero() && p.lastCheck.Add(time.Hour).After(time.Now()) {
		return p.latestUpdate, nil
	}

	// Get local architecture.
	archName, err := osarch.ArchitectureGetLocal()
	if err != nil {
		return nil, err
	}

	index := &apiupdate.Index{}

	if !p.ignoreSignedJSON { //nolint:nestif
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

		// Validate signed index.
		verified, err := util.VerifySMIME(ctx, p.updateCA, resp.Body)
		if err != nil {
			return nil, err
		}

		// Parse the update list.
		err = json.NewDecoder(bytes.NewReader(verified.Bytes())).Decode(index)
		if err != nil {
			return nil, err
		}
	} else {
		if !strings.HasPrefix(p.serverURL, "https://") {
			return nil, errors.New("cannot disable JSON metadata verification for requests made via HTTP")
		}

		// Get the latest index.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.serverURL+"/index.json", nil)
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

		// Parse the update list.
		err = json.NewDecoder(resp.Body).Decode(index)
		if err != nil {
			return nil, err
		}
	}

	// Get the latest update for the expected channel.
	var latestUpdate *apiupdate.UpdateFull

	channelExists := false

	for _, update := range index.Updates {
		// Skip any update targeting the wrong channel(s).
		if p.state.System.Update.Config.Channel != "" && !slices.Contains(update.Channels, p.state.System.Update.Config.Channel) {
			// If dealing with an image other than the current one, skip.
			if update.Version != p.state.OS.RunningRelease {
				continue
			}
		} else {
			// Record that we found the channel in the remote list.
			channelExists = true
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

	if !channelExists {
		slog.Warn("The configured update channel doesn't currently hold any image", "channel", p.state.System.Update.Config.Channel)
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
		if filepath.Base(file.Filename) != a.name+".raw.gz" || file.Type != apiupdate.UpdateFileTypeApplication {
			continue
		}

		fileURL := a.provider.serverURL + "/" + a.latestUpdate.Version + "/" + file.Filename
		targetName := strings.TrimSuffix(filepath.Base(file.Filename), ".gz")

		// If the application sysext image already exists on disk, don't re-download it.
		_, err := os.Stat(filepath.Join(targetPath, targetName))
		if err == nil {
			continue
		}

		// Download the application.
		err = downloadAsset(ctx, a.provider.state.OS.Name, a.provider.state.OS.RunningRelease, a.provider.client, fileURL, file.Sha256, filepath.Join(targetPath, targetName), progressFunc)
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
		err = downloadAsset(ctx, o.provider.state.OS.Name, o.provider.state.OS.RunningRelease, o.provider.client, fileURL, file.Sha256, filepath.Join(targetPath, targetName), progressFunc)
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
		err = downloadAsset(ctx, o.provider.state.OS.Name, o.provider.state.OS.RunningRelease, o.provider.client, fileURL, file.Sha256, filepath.Join(targetPath, targetName), progressFunc)

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
		err = downloadAsset(ctx, o.provider.state.OS.Name, o.provider.state.OS.RunningRelease, o.provider.client, fileURL, file.Sha256, filepath.Join(targetPath, o.GetFilename()), nil)
		if err != nil {
			return fmt.Errorf("while downloading %s, got error '%s'", fileURL, err.Error())
		}
	}

	return nil
}
