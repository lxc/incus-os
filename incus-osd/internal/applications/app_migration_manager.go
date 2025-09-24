package applications

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/FuturFusion/migration-manager/shared/api"

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
	// Reload the systemd daemon to pickup any service definition changes.
	err := systemd.ReloadDaemon(ctx)
	if err != nil {
		return err
	}

	// Restart the unit.
	return systemd.RestartUnit(ctx, "migration-manager.service")
}

// Initialize runs first time initialization.
func (*migrationManager) Initialize(ctx context.Context) error {
	// Get the preseed from the seed partition.
	mmSeed, err := seed.GetMigrationManager(ctx, seed.GetSeedPath())
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	// Return if no seed was provided.
	if mmSeed == nil {
		return nil
	}

	// Wait for Migration Manager to begin accepting connections.
	count := 0

	for {
		err := doMMRequest(ctx, "http://localhost/1.0", http.MethodGet, nil)
		if err == nil {
			break
		}

		count++

		if count > 10 {
			return errors.New("failed to connect to Migration Manager via local socket")
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Apply SystemCertificate, if any.
	if mmSeed.SystemCertificate != nil {
		contentJSON, err := json.Marshal(mmSeed.SystemCertificate)
		if err != nil {
			return err
		}

		err = doMMRequest(ctx, "http://localhost/1.0/system/certificate", http.MethodPost, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemNetwork, if any.
	if mmSeed.SystemNetwork == nil {
		mmSeed.SystemNetwork = new(api.SystemNetwork)
	}

	{
		// If no IP address is provided, default to listening on all addresses with the default port.
		if mmSeed.SystemNetwork.Address == "" {
			mmSeed.SystemNetwork.Address = "[::]"
		}

		contentJSON, err := json.Marshal(mmSeed.SystemNetwork)
		if err != nil {
			return err
		}

		err = doMMRequest(ctx, "http://localhost/1.0/system/network", http.MethodPut, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemSecurity, if any.
	if mmSeed.SystemSecurity == nil && len(mmSeed.TrustedClientCertificates) > 0 {
		mmSeed.SystemSecurity = new(api.SystemSecurity)
	}

	if mmSeed.SystemSecurity != nil {
		// Compute fingerprints for any user-provided client certificates and add to the
		// list of trusted TLS client certificates.
		for i, certString := range mmSeed.TrustedClientCertificates {
			certBlock, _ := pem.Decode([]byte(certString))
			if certBlock == nil {
				return fmt.Errorf("cannot parse client certificate PEM in seed (index %d)", i)
			}

			cert, err := x509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				return fmt.Errorf("invalid client certificate in seed (index %d): %w", i, err)
			}

			rawFp := sha256.Sum256(cert.Raw)
			fp := hex.EncodeToString(rawFp[:])

			if !slices.Contains(mmSeed.SystemSecurity.TrustedTLSClientCertFingerprints, fp) {
				mmSeed.SystemSecurity.TrustedTLSClientCertFingerprints = append(mmSeed.SystemSecurity.TrustedTLSClientCertFingerprints, fp)
			}
		}

		contentJSON, err := json.Marshal(mmSeed.SystemSecurity)
		if err != nil {
			return err
		}

		err = doMMRequest(ctx, "http://localhost/1.0/system/security", http.MethodPut, contentJSON)
		if err != nil {
			return err
		}
	}

	return nil
}

// IsRunning reports if the application is currently running.
func (*migrationManager) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "migration-manager.service")
}

// Migration Manager specific helper to interact with the REST API.
func doMMRequest(ctx context.Context, url string, method string, body []byte) error {
	client, err := unixHTTPClient("/run/migration-manager/unix.socket")
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return getErrorMessage(resp.Body)
	}

	return nil
}

// IsPrimary reports if the application is a primary application.
func (*migrationManager) IsPrimary() bool {
	return true
}
