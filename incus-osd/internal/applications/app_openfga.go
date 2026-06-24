package applications

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/lxc/incus/v7/shared/subprocess"
	"go.yaml.in/yaml/v4"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/zfs"
)

type openfga struct {
	common
}

type openfgaConfig struct {
	Authn openfgaConfigAuthn `yaml:"authn"`
	Grpc  openfgaConfigGrpc  `yaml:"grpc"`
	HTTP  openfgaConfigHTTP  `yaml:"http"`
}

type openfgaConfigAuthn struct {
	Method    string                      `yaml:"method"`
	Preshared openfgaConfigAuthnPreshared `yaml:"preshared"`
}

type openfgaConfigAuthnPreshared struct {
	Keys []string `yaml:"keys"`
}

type openfgaConfigGrpc struct {
	Addr string           `yaml:"addr"`
	TLS  openfgaConfigTLS `yaml:"tls"`
}

type openfgaConfigHTTP struct {
	Addr string           `yaml:"addr"`
	TLS  openfgaConfigTLS `yaml:"tls"`
}

type openfgaConfigTLS struct {
	Enabled bool   `yaml:"enabled"`
	Cert    string `yaml:"cert"`
	Key     string `yaml:"key"`
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

	// Clear any existing application state.
	o.state.Applications.OpenFGA.State.Initialized = false
	o.state.Applications.OpenFGA.Config = api.ApplicationOpenFGAConfig{}

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
	return o.UpdateConfig(ctx, initialToken)
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
func (o *openfga) RestoreBackup(archive io.Reader) error {
	err := extractTarArchive("/var/lib/openfga/", []string{"openfga.service"}, archive)
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
	// Ensure local storage for OpenFGA is configured before doing anything else.
	err := o.ConfigureLocalStorage(ctx)
	if err != nil {
		return err
	}

	// Each time OpenFGA starts, grab a copy of the primary application's TLS certificate and
	// key so OpenFGA can use the same TLS certificate when serving requests. This also simplifies
	// handling of TLS certificate rotation, as OpenFGA only needs to be restarted to pickup the change.
	primaryApp, err := GetPrimary(ctx, o.state, true)
	if err != nil {
		return err
	}

	tlsCert, err := primaryApp.GetServerCertificate()
	if err != nil {
		return err
	}

	err = writeCerts(tlsCert)
	if err != nil {
		return err
	}

	// Start the unit.
	return systemd.StartUnit(ctx, "openfga.service")
}

func (*openfga) StartupWeight() int {
	return 1
}

func writeCerts(tlsCert *tls.Certificate) error {
	var buf bytes.Buffer

	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(tlsCert.PrivateKey)
	if err != nil {
		return err
	}

	block := pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privKeyBytes,
	}

	err = pem.Encode(&buf, &block)
	if err != nil {
		return err
	}

	err = os.WriteFile("/var/lib/openfga/server.key", buf.Bytes(), 0o400)
	if err != nil {
		return err
	}

	buf.Reset()

	for _, cert := range tlsCert.Certificate {
		block := pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		}

		err := pem.Encode(&buf, &block)
		if err != nil {
			return err
		}
	}

	err = os.WriteFile("/var/lib/openfga/server.crt", buf.Bytes(), 0o644)
	if err != nil {
		return err
	}

	return nil
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
			Addr: "localhost:55123", // Since we can't disable the gRPC service, bind to localhost only on a high port.
			TLS: openfgaConfigTLS{
				Enabled: true,
				Cert:    "/var/lib/openfga/server.crt",
				Key:     "/var/lib/openfga/server.key",
			},
		},
		HTTP: openfgaConfigHTTP{
			Addr: ":8444",
			TLS: openfgaConfigTLS{
				Enabled: true,
				Cert:    "/var/lib/openfga/server.crt",
				Key:     "/var/lib/openfga/server.key",
			},
		},
	}

	// Dump configuration to yaml.
	contents, err := yaml.Dump(&cfg, yaml.WithV2Defaults())
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

	return nil
}
