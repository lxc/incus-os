package applications

import (
	"context"

	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type openfga struct{}

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

// InitializePreStart runs first time initialization before the application starts.
func (*openfga) InitializePreStart(_ context.Context) error {
	return nil
}

// InitializePostStart runs first time initialization after the application starts.
func (*openfga) InitializePostStart(_ context.Context) error {
	return nil
}

// IsRunning reports if the application is currently running.
func (*openfga) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "openfga.service")
}
