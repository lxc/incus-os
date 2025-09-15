package applications

import (
	"context"
	"crypto/tls"
	"errors"
	"io"

	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type openfga struct {
	common //nolint:unused
}

// Start starts the systemd unit.
func (*openfga) Start(ctx context.Context, _ string) error {
	// Start the unit.
	return systemd.EnableUnit(ctx, true, "openfga.service")
}

// Stop stops the systemd unit.
func (*openfga) Stop(ctx context.Context, _ string) error {
	// Stop the unit.
	return systemd.StopUnit(ctx, "openfga.service")
}

// Update triggers restart after an application update.
func (*openfga) Update(ctx context.Context, _ string) error {
	// Reload the systemd daemon to pickup any service definition changes.
	err := systemd.ReloadDaemon(ctx)
	if err != nil {
		return err
	}

	// Restart the unit.
	return systemd.RestartUnit(ctx, "openfga.service")
}

// Initialize runs first time initialization.
func (*openfga) Initialize(_ context.Context) error {
	return nil
}

// IsRunning reports if the application is currently running.
func (*openfga) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "openfga.service")
}

// IsPrimary reports if the application is a primary application.
func (*openfga) IsPrimary() bool {
	return false
}

// GetCertificate returns the keypair for the server certificate.
func (*openfga) GetCertificate() (*tls.Certificate, error) {
	return nil, errors.New("not supported")
}

// AddTrustedCertificate adds a new trusted certificate to the application.
func (*openfga) AddTrustedCertificate(_ context.Context, _ string, _ string) error {
	return errors.New("not supported")
}

// WipeLocalData removes local data created by the application.
func (*openfga) WipeLocalData() error {
	return errors.New("not supported")
}

// FactoryReset performs a full factory reset of the application.
func (*openfga) FactoryReset(_ context.Context) error {
	return errors.New("not supported")
}

// GetBackup returns a tar archive backup of the application's configuration and/or state.
func (*openfga) GetBackup(_ io.Writer, _ bool) error {
	return errors.New("not supported")
}

// RestoreBackup restores a tar archive backup of the application's configuration and/or state.
func (*openfga) RestoreBackup(_ io.Reader) error {
	return errors.New("not supported")
}
