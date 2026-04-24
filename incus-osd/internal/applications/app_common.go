package applications

import (
	"context"
	"crypto/tls"
	"errors"
	"io"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

type common struct {
	state *state.State
}

// AddTrustedCertificate adds a new trusted certificate to the application.
func (*common) AddTrustedCertificate(_ context.Context, _ string, _ string) error {
	return errors.New("not supported")
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
func (*common) Initialize(_ context.Context) error {
	return nil
}

// IsPrimary reports if the application is a primary application.
func (*common) IsPrimary() bool {
	return false
}

// IsRunning reports if the application is currently running.
func (*common) IsRunning(_ context.Context) bool {
	return false
}

func (*common) Name() string {
	return ""
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

// Stop runs shutdown action.
func (*common) Stop(_ context.Context) error {
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
