package applications

import (
	"context"

	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type operationsCenter struct{}

// Start starts the systemd unit.
func (*operationsCenter) Start(ctx context.Context, _ string) error {
	// Start the unit.
	return systemd.EnableUnit(ctx, true, "operations-center.service")
}

// Stop stops the systemd unit.
func (*operationsCenter) Stop(ctx context.Context, _ string) error {
	// Stop the unit.
	return systemd.StopUnit(ctx, "operations-center.service")
}

// Update triggers restart after an application update.
func (*operationsCenter) Update(ctx context.Context, _ string) error {
	// Restart the unit.
	return systemd.RestartUnit(ctx, "operations-center.service")
}

// Initialize runs first time initialization.
func (*operationsCenter) Initialize(_ context.Context) error {
	return nil
}

// IsRunning reports if the application is currently running.
func (*operationsCenter) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "operations-center.service")
}
