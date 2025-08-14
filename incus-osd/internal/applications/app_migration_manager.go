package applications

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
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
func (*migrationManager) Initialize(ctx context.Context) error {
	// Get the preseed from the seed partition.
	mmSeed, err := seed.GetMigrationManager(ctx, seed.SeedPartitionPath)
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	// Return if no seed was provided.
	if mmSeed == nil {
		return nil
	}

	// Apply provided server certificate.
	if mmSeed.ServerCertificate != "" && mmSeed.ServerKey != "" {
		_, err := tls.X509KeyPair([]byte(mmSeed.ServerCertificate), []byte(mmSeed.ServerKey))
		if err != nil {
			return fmt.Errorf("failed to validate server key pair: %w", err)
		}

		err = os.MkdirAll("/var/lib/migration-manager/", 0o755)
		if err != nil {
			return fmt.Errorf("failed to create directory /var/lib/migration-manager/: %w", err)
		}

		err = os.WriteFile("/var/lib/migration-manager/server.crt", []byte(mmSeed.ServerCertificate), 0o644)
		if err != nil {
			return fmt.Errorf("failed to write /var/lib/migration-manager/server.crt: %w", err)
		}

		err = os.WriteFile("/var/lib/migration-manager/server.key", []byte(mmSeed.ServerKey), 0o600)
		if err != nil {
			return fmt.Errorf("failed to write /var/lib/migration-manager/server.key: %w", err)
		}
	}

	// Dump any other preseed fields to config file.
	// Ensure the certificate fields are empty, since we've already written any contents to disk.
	mmSeed.ServerCertificate = ""
	mmSeed.ServerKey = ""

	contents, err := yaml.Marshal(mmSeed)
	if err != nil {
		return err
	}

	return os.WriteFile("/var/lib/migration-manager/config.yml", contents, 0o644)
}

// IsRunning reports if the application is currently running.
func (*migrationManager) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "migration-manager.service")
}
