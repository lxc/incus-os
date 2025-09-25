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

	"github.com/FuturFusion/operations-center/shared/api"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type operationsCenter struct {
	common
}

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
	ocSeed, err := seed.GetOperationsCenter(ctx, seed.GetSeedPath())
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	// Return if no seed was provided.
	if ocSeed == nil {
		return nil
	}

	// Wait for Operations Center to begin accepting connections.
	count := 0

	for {
		err := doOCRequest(ctx, "http://localhost/1.0", http.MethodGet, nil)
		if err == nil {
			break
		}

		count++

		if count > 10 {
			return errors.New("failed to connect to Operations Center via local socket")
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Apply SystemCertificate, if any.
	if ocSeed.SystemCertificate != nil {
		contentJSON, err := json.Marshal(ocSeed.SystemCertificate)
		if err != nil {
			return err
		}

		err = doOCRequest(ctx, "http://localhost/1.0/system/certificate", http.MethodPost, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemNetwork, if any.
	if ocSeed.SystemNetwork == nil {
		ocSeed.SystemNetwork = new(api.SystemNetworkPut)
	}

	{
		// If no IP address is provided, default to listening on all addresses with the default port.
		if ocSeed.SystemNetwork.OperationsCenterAddress == "" && ocSeed.SystemNetwork.RestServerAddress == "" {
			ocSeed.SystemNetwork.OperationsCenterAddress = "https://[::]:0"
			ocSeed.SystemNetwork.RestServerAddress = "[::]:0"
		}

		contentJSON, err := json.Marshal(ocSeed.SystemNetwork)
		if err != nil {
			return err
		}

		err = doOCRequest(ctx, "http://localhost/1.0/system/network", http.MethodPut, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemSecurity, if any.
	if ocSeed.SystemSecurity == nil && len(ocSeed.TrustedClientCertificates) > 0 {
		ocSeed.SystemSecurity = new(api.SystemSecurityPut)
	}

	if ocSeed.SystemSecurity != nil {
		// Compute fingerprints for any user-provided client certificates and add to the
		// list of trusted TLS client certificates.
		for i, certString := range ocSeed.TrustedClientCertificates {
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

			if !slices.Contains(ocSeed.SystemSecurity.TrustedTLSClientCertFingerprints, fp) {
				ocSeed.SystemSecurity.TrustedTLSClientCertFingerprints = append(ocSeed.SystemSecurity.TrustedTLSClientCertFingerprints, fp)
			}
		}

		contentJSON, err := json.Marshal(ocSeed.SystemSecurity)
		if err != nil {
			return err
		}

		err = doOCRequest(ctx, "http://localhost/1.0/system/security", http.MethodPut, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemUpdates, if any.
	if ocSeed.SystemUpdates != nil {
		contentJSON, err := json.Marshal(ocSeed.SystemUpdates)
		if err != nil {
			return err
		}

		err = doOCRequest(ctx, "http://localhost/1.0/system/updates", http.MethodPut, contentJSON)
		if err != nil {
			return err
		}
	}

	// Restart the service to ensure any seed settings are properly applied.
	return systemd.RestartUnit(ctx, "operations-center.service")
}

// IsRunning reports if the application is currently running.
func (*operationsCenter) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "operations-center.service")
}

// Operations Center specific helper to interact with the REST API.
func doOCRequest(ctx context.Context, url string, method string, body []byte) error {
	client, err := unixHTTPClient("/run/operations-center/unix.socket")
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
func (*operationsCenter) IsPrimary() bool {
	return true
}
