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
	// Reload the systemd daemon to pickup any service definition changes.
	err := systemd.ReloadDaemon(ctx)
	if err != nil {
		return err
	}

	// Restart the unit.
	return systemd.RestartUnit(ctx, "operations-center.service")
}

// Initialize runs first time initialization.
func (*operationsCenter) Initialize(ctx context.Context) error {
	// Get the preseed from the seed partition.
	ocSeed, err := seed.GetOperationsCenter(ctx, seed.SeedPartitionPath)
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	// Return if no seed was provided.
	if ocSeed == nil {
		return nil
	}

	// Apply provided server certificate.
	if ocSeed.ServerCertificate != "" && ocSeed.ServerKey != "" {
		_, err := tls.X509KeyPair([]byte(ocSeed.ServerCertificate), []byte(ocSeed.ServerKey))
		if err != nil {
			return fmt.Errorf("failed to validate server key pair: %w", err)
		}

		err = os.MkdirAll("/var/lib/operations-center/", 0o755)
		if err != nil {
			return fmt.Errorf("failed to create directory /var/lib/operations-center/: %w", err)
		}

		err = os.WriteFile("/var/lib/operations-center/server.crt", []byte(ocSeed.ServerCertificate), 0o644)
		if err != nil {
			return fmt.Errorf("failed to write /var/lib/operations-center/server.crt: %w", err)
		}

		err = os.WriteFile("/var/lib/operations-center/server.key", []byte(ocSeed.ServerKey), 0o600)
		if err != nil {
			return fmt.Errorf("failed to write /var/lib/operations-center/server.key: %w", err)
		}
	}

	// Apply provided client certificate.
	if ocSeed.ClientCertificate != "" && ocSeed.ClientKey != "" {
		_, err := tls.X509KeyPair([]byte(ocSeed.ClientCertificate), []byte(ocSeed.ClientKey))
		if err != nil {
			return fmt.Errorf("failed to validate client key pair: %w", err)
		}

		err = os.MkdirAll("/var/lib/operations-center/", 0o755)
		if err != nil {
			return fmt.Errorf("failed to create directory /var/lib/operations-center/: %w", err)
		}

		err = os.WriteFile("/var/lib/operations-center/client.crt", []byte(ocSeed.ClientCertificate), 0o644)
		if err != nil {
			return fmt.Errorf("failed to write /var/lib/operations-center/client.crt: %w", err)
		}

		err = os.WriteFile("/var/lib/operations-center/client.key", []byte(ocSeed.ClientKey), 0o600)
		if err != nil {
			return fmt.Errorf("failed to write /var/lib/operations-center/client.key: %w", err)
		}
	}

	// Dump any other preseed fields to config file.
	// Ensure the certificate fields are empty, since we've already written any contents to disk.
	ocSeed.ServerCertificate = ""
	ocSeed.ServerKey = ""
	ocSeed.ClientCertificate = ""
	ocSeed.ClientKey = ""

	contents, err := yaml.Marshal(ocSeed)
	if err != nil {
		return err
	}

	return os.WriteFile("/var/lib/operations-center/config.yml", contents, 0o644)
}

// IsRunning reports if the application is currently running.
func (*operationsCenter) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "operations-center.service")
}
