package applications

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"slices"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

type common struct {
	state *state.State

	appState *api.ApplicationState
}

// AddTrustedCertificate adds a new trusted certificate to the application.
func (*common) AddTrustedCertificate(_ context.Context, _ string, _ string) error {
	return errors.New("not supported")
}

// AvailableVersions returns a list of available versions.
func (a *common) AvailableVersions() []string {
	return a.appState.AvailableVersions
}

// ConfigureLocalStorage configures local storage for the application.
func (*common) ConfigureLocalStorage(_ context.Context) error {
	return nil
}

// Debug runs a debug action.
func (*common) Debug(_ context.Context, _ any) response.Response {
	return response.NotImplemented(nil)
}

// DebugStruct returns the struct to fill with debug request data.
func (*common) DebugStruct() any {
	var data any

	return &data
}

// FactoryReset performs a full factory reset of the application.
func (*common) FactoryReset(_ context.Context) error {
	return errors.New("not supported")
}

// GetBackup returns a tar archive backup of the application's configuration and/or state.
func (*common) GetBackup(_ io.Writer, _ bool) error {
	return errors.New("not supported")
}

// GetClientCertificate gets the client certificate for the application.
// That is, the client certificate that the application would use when
// it needs to authenticate itself with a 3rd party service (like a provider).
func (*common) GetClientCertificate() (*tls.Certificate, error) {
	return nil, errors.New("not supported")
}

// GetDependencies returns a list of other applications this application depends on.
func (*common) GetDependencies() []string {
	return nil
}

// GetServerCertificate gets the server certificate for the application.
// That is, the certificate which will be used when a user connects to
// the public port for the application.
func (*common) GetServerCertificate() (*tls.Certificate, error) {
	return nil, errors.New("not supported")
}

// Initialize runs first time initialization.
func (a *common) Initialize(_ context.Context) error {
	a.appState.Initialized = true

	return nil
}

// IsInstalled reports whether the application has been installed.
func (a *common) IsInstalled() bool {
	return a.appState.Version != ""
}

// IsInitialized reports whether the application has been initialized.
func (a *common) IsInitialized() bool {
	return a.appState.Initialized
}

// IsPrimary reports if the application is a primary application.
func (*common) IsPrimary() bool {
	return false
}

// IsRunning reports if the application is currently running.
func (*common) IsRunning(_ context.Context) bool {
	return false
}

// NeedsLateUpdateCheck reports if the application depends on a delayed provider update check.
func (*common) NeedsLateUpdateCheck() bool {
	return false
}

// Restart restarts runs restart action.
func (*common) Restart(_ context.Context) error {
	return nil
}

// RestoreBackup restores a tar archive backup of the application's configuration and/or state.
func (*common) RestoreBackup(_ context.Context, _ io.Reader) error {
	return errors.New("not supported")
}

// Start runs startup action.
func (*common) Start(_ context.Context) error {
	return nil
}

// SetVersions sets the actual and available versions for the application.
func (a *common) SetVersions(version string, availableVersions []string) {
	a.appState.Version = version
	a.appState.AvailableVersions = availableVersions
}

// Stop runs shutdown action.
func (*common) Stop(_ context.Context) error {
	return nil
}

// SwitchVersion attempts to change the configured version of the application. If no version is specified,
// try to rollback to the prior available version of the application, if available.
//
// Note that the underlying sysext images must be refreshed to actually update the running version of the application.
func (a *common) SwitchVersion(newVersion string) error {
	// Check if it's possible to change the application version.
	if len(a.appState.AvailableVersions) == 1 {
		return errors.New("only one version of the application is available locally, cannot switch to a different version")
	}

	// Determine the new application version.
	var version string

	if newVersion == "" {
		versionIndex := slices.Index(a.appState.AvailableVersions, a.appState.Version)
		if versionIndex == 0 {
			return errors.New("cannot rollback application as no earlier version is available locally")
		}

		version = a.appState.AvailableVersions[versionIndex-1]
	} else {
		if !slices.Contains(a.appState.AvailableVersions, newVersion) {
			return errors.New("cannot switch application to version '" + newVersion + "' because it does not exist locally")
		}

		version = newVersion
	}

	// Update application's version.
	a.appState.Version = version

	return nil
}

// Update triggers a partial application restart after an update.
func (*common) Update(_ context.Context) error {
	return nil
}

// WipeLocalData removes local data created by the application.
func (*common) WipeLocalData(_ context.Context) error {
	return nil
}

// Version returns the installed version.
func (a *common) Version() string {
	return a.appState.Version
}
