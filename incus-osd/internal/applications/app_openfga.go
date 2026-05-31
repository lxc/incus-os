package applications

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/lxc/incus/v7/shared/subprocess"
	"go.yaml.in/yaml/v4"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/zfs"
)

type openfga struct {
	common
}

type openfgaConfig struct {
	Authn   openfgaConfigAuthn   `yaml:"authn"`
	Grpc    openfgaConfigGrpc    `yaml:"grpc"`
	HTTP    openfgaConfigHTTP    `yaml:"http"`
	Metrics openfgaConfigMetrics `yaml:"metrics"`
}

type openfgaConfigAuthn struct {
	Method    string                      `yaml:"method"`
	Preshared openfgaConfigAuthnPreshared `yaml:"preshared"`
}

type openfgaConfigAuthnPreshared struct {
	Keys []string `yaml:"keys"`
}

type openfgaConfigGrpc struct {
	Addr string `yaml:"addr"`
}

type openfgaConfigHTTP struct {
	Addr string `yaml:"addr"`
}

type openfgaConfigMetrics struct {
	Addr string `yaml:"addr"`
}

// ConfigureLocalStorage configures local storage for the application.
func (o *openfga) ConfigureLocalStorage(ctx context.Context) error {
	// If the application isn't initialized, create a ZFS dataset for it to use.
	if !o.IsInitialized() {
		err := zfs.CreateApplicationDataset(ctx, "openfga")
		if err != nil {
			return err
		}
	} else {
		err := zfs.MountApplicationDataset(ctx, "openfga")
		if err != nil {
			return err
		}
	}

	return nil
}

// FactoryReset performs a full factory reset of the application.
func (o *openfga) FactoryReset(ctx context.Context) error {
	// Stop the application.
	err := o.Stop(ctx)
	if err != nil {
		return err
	}

	// Wipe local configuration.
	err = o.WipeLocalData(ctx)
	if err != nil {
		return err
	}

	// Start the application.
	err = o.Start(ctx)
	if err != nil {
		return err
	}

	// Perform first start initialization.
	return o.Initialize(ctx)
}

func (o *openfga) Get(_ context.Context) (any, error) {
	return o.state.Applications.OpenFGA, nil
}

// GetBackup returns a tar archive backup of the application's configuration and/or state.
func (*openfga) GetBackup(archive io.Writer, _ bool) error {
	return createTarArchive("/var/lib/openfga/", nil, archive)
}

// GetDependencies returns a list of other applications this application depends on.
func (*openfga) GetDependencies() []string {
	return nil
}

// Initialize runs first time initialization.
func (o *openfga) Initialize(ctx context.Context) error {
	// Ensure the default configuration directory exists.
	err := os.Mkdir("/etc/openfga/", 0o755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Set an initial random authentication token.
	initialToken := &api.ApplicationOpenFGA{
		Config: api.ApplicationOpenFGAConfig{
			APITokens: []string{rand.Text()},
		},
	}

	// Mark application as initialized.
	o.appState.Initialized = true

	// Apply the configuration and restart the service.
	err = o.UpdateConfig(ctx, initialToken)
	if err != nil {
		return err
	}

	// Create a default store and record its ID in the application state.
	return createDefaultStore(ctx, o.state)
}

// IsInstalled reports whether the application has been installed.
func (o *openfga) IsInstalled() bool {
	if o.appState.Version == "" {
		return false
	}

	return sysextImageExists(o.Name(), o.appState.Version)
}

// IsPrimary reports if the application is a primary application.
func (*openfga) IsPrimary() bool {
	return false
}

// IsRunning reports if the application is currently running.
func (*openfga) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "openfga.service")
}

func (*openfga) Name() string {
	return "openfga"
}

// NeedsLateUpdateCheck reports if the application depends on a delayed provider update check.
func (*openfga) NeedsLateUpdateCheck() bool {
	return false
}

// Restart restarts the main systemd unit.
func (*openfga) Restart(ctx context.Context) error {
	return systemd.RestartUnit(ctx, "openfga.service")
}

// RestoreBackup restores a tar archive backup of the application's configuration and/or state.
func (o *openfga) RestoreBackup(ctx context.Context, archive io.Reader) error {
	err := extractTarArchive(ctx, "/var/lib/openfga/", []string{"openfga.service"}, archive)
	if err != nil {
		return err
	}

	// Restore any configured trust tokens to application state.
	contents, err := os.ReadFile("/var/lib/openfga/config.yaml")
	if err != nil {
		return err
	}

	cfg := openfgaConfig{}

	err = yaml.Load(contents, &cfg)
	if err != nil {
		return err
	}

	o.state.Applications.OpenFGA.Config.APITokens = cfg.Authn.Preshared.Keys

	// Ensure the default configuration directory exists.
	err = os.Mkdir("/etc/openfga/", 0o755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Ensure a symlinked configuration file exists where openfga will look for it.
	_, err = os.Lstat("/etc/openfga/config.yaml")
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		err := os.Symlink("/var/lib/openfga/config.yaml", "/etc/openfga/config.yaml")
		if err != nil {
			return err
		}
	}

	// Record when the application was restored.
	now := time.Now()
	o.appState.LastRestored = &now

	return nil
}

// SetFriendlyVersion records the friendly version.
func (o *openfga) SetFriendlyVersion(ctx context.Context) error {
	_, stderr, err := subprocess.RunCommandSplit(ctx, nil, nil, "openfga", "version")
	if err != nil {
		return err
	}

	versionRegex := regexp.MustCompile("OpenFGA version `(.+)` build from")
	versionGroup := versionRegex.FindStringSubmatch(stderr)

	if len(versionGroup) != 2 {
		return errors.New("unable to determine OpenFGA version")
	}

	o.appState.FriendlyVersion = versionGroup[1] + " [" + o.appState.Version + "]"

	return nil
}

// Start starts the systemd unit.
func (o *openfga) Start(ctx context.Context) error {
	err := o.ConfigureLocalStorage(ctx)
	if err != nil {
		return err
	}

	// Start the unit.
	return systemd.StartUnit(ctx, "openfga.service")
}

// Stop stops the systemd unit.
func (*openfga) Stop(ctx context.Context) error {
	// Stop the unit.
	return systemd.StopUnit(ctx, "openfga.service")
}

func (*openfga) Struct() any {
	return &api.ApplicationOpenFGA{}
}

// Update triggers restart after an application update.
func (*openfga) Update(ctx context.Context) error {
	// Reload the systemd daemon to pickup any service definition changes.
	err := systemd.ReloadDaemon(ctx)
	if err != nil {
		return err
	}

	// Restart the unit.
	return systemd.RestartUnit(ctx, "openfga.service")
}

func (o *openfga) UpdateConfig(ctx context.Context, req any) error {
	newState, ok := req.(*api.ApplicationOpenFGA)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ApplicationOpenFGA", req)
	}

	if len(newState.Config.APITokens) == 0 {
		return errors.New("at least one API token must be specified")
	}

	// Update the configuration.
	o.state.Applications.OpenFGA.Config = newState.Config

	// Remove any existing configuration.
	err := os.Remove("/var/lib/openfga/config.yaml")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	cfg := openfgaConfig{
		Authn: openfgaConfigAuthn{
			Method: "preshared",
			Preshared: openfgaConfigAuthnPreshared{
				Keys: o.state.Applications.OpenFGA.Config.APITokens,
			},
		},
		Grpc: openfgaConfigGrpc{
			Addr: "127.0.0.1:8081",
		},
		HTTP: openfgaConfigHTTP{
			Addr: "127.0.0.1:8080",
		},
		Metrics: openfgaConfigMetrics{
			Addr: "127.0.0.1:2112",
		},
	}

	// Dump configuration to yaml.
	contents, err := yaml.Dump(&cfg, yaml.V2)
	if err != nil {
		return err
	}

	// Write the new configuration file.
	err = os.WriteFile("/var/lib/openfga/config.yaml", contents, 0o600)
	if err != nil {
		return err
	}

	// Ensure a symlinked configuration file exists where openfga will look for it.
	_, err = os.Lstat("/etc/openfga/config.yaml")
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		err := os.Symlink("/var/lib/openfga/config.yaml", "/etc/openfga/config.yaml")
		if err != nil {
			return err
		}
	}

	// Save the state.
	err = o.state.Save()
	if err != nil {
		return err
	}

	// Restart the application.
	return o.Update(ctx)
}

// WipeLocalData removes local data created by the application.
func (*openfga) WipeLocalData(ctx context.Context) error {
	// Remove configuration file symlink.
	err := os.Remove("/etc/openfga/config.yaml")
	if err != nil {
		return err
	}

	// Unmount the dataset.
	err = unix.Unmount("/var/lib/openfga/", 0)
	if err != nil {
		return err
	}

	// Destroy the dataset.
	err = zfs.DestroyDataset(ctx, "local", "openfga", false)
	if err != nil {
		return err
	}

	// For good measure, remove the mount point.
	err = os.RemoveAll("/var/lib/openfga/")
	if err != nil {
		return err
	}

	// Create a fresh dataset for the application.
	return zfs.CreateApplicationDataset(ctx, "openfga")
}

func createDefaultStore(ctx context.Context, s *state.State) error {
	type openfgaResponse struct {
		ID string `json:"id"`
	}

	client := &http.Client{}

	body := `{"name": "` + s.OS.Name + `"}`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:8080/stores", strings.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+s.Applications.OpenFGA.Config.APITokens[0])

	count := 0

	var resp *http.Response

	// Perform the request in a loop to allow time for the service to become available.
	for {
		resp, err = client.Do(req)
		if err != nil {
			if !errors.Is(err, syscall.ECONNREFUSED) {
				return err
			}

			if count > 10 {
				return errors.New("unable to connect to openfga service")
			}

			count++

			time.Sleep(50 * time.Millisecond)

			continue
		}

		break
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return errors.New("failed to create default store")
	}

	r := &openfgaResponse{}

	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return err
	}

	s.Applications.OpenFGA.State.StoreID = r.ID

	return nil
}
