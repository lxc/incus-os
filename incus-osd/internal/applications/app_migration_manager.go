package applications

import (
	"context"

	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type migrationManager struct{}

// Start starts the systemd unit.
func (*migrationManager) Start(ctx context.Context, _ string) error {
	// Start the unit.
	return systemd.EnableUnit(ctx, true, "migration-manager.service")
}

// Stop stops the systemd unit.
func (*migrationManager) Stop(ctx context.Context, _ string) error {
	// Stop the unit.
	return systemd.StopUnit(ctx, "migration-manager.service")
}

// Update triggers restart after an application update.
func (*migrationManager) Update(ctx context.Context, _ string) error {
	// Restart the unit.
	return systemd.RestartUnit(ctx, "migration-manager.service")
}

// Initialize runs first time initialization.
func (*migrationManager) Initialize(_ context.Context) error {
	return nil
}

// IsRunning reports if the application is currently running.
func (*migrationManager) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "migration-manager.service")
}
