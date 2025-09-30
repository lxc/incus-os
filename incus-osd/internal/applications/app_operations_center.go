package applications

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/FuturFusion/operations-center/shared/api"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type operationsCenter struct {
	common //nolint:unused
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

	// Configure an empty seed if none was provided.
	if ocSeed == nil {
		ocSeed = new(apiseed.OperationsCenter)
	}

	// Wait for Operations Center to begin accepting connections.
	count := 0

	for {
		_, err := doOCRequest(ctx, "http://localhost/1.0", http.MethodGet, nil)
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

		_, err = doOCRequest(ctx, "http://localhost/1.0/system/certificate", http.MethodPost, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemNetwork, if any.
	if ocSeed.SystemNetwork == nil {
		ocSeed.SystemNetwork = new(api.SystemNetworkPut)
	}

	{
		// If no IP address is provided, default to listening on all addresses on port 8443.
		if ocSeed.SystemNetwork.OperationsCenterAddress == "" && ocSeed.SystemNetwork.RestServerAddress == "" {
			ocSeed.SystemNetwork.OperationsCenterAddress = "https://[::]:8443"
			ocSeed.SystemNetwork.RestServerAddress = "[::]:8443"
		}

		contentJSON, err := json.Marshal(ocSeed.SystemNetwork)
		if err != nil {
			return err
		}

		_, err = doOCRequest(ctx, "http://localhost/1.0/system/network", http.MethodPut, contentJSON)
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
			fp, err := getCertificateFingerprint(certString)
			if err != nil {
				return fmt.Errorf("%w (seed index %d)", err, i)
			}

			if !slices.Contains(ocSeed.SystemSecurity.TrustedTLSClientCertFingerprints, fp) {
				ocSeed.SystemSecurity.TrustedTLSClientCertFingerprints = append(ocSeed.SystemSecurity.TrustedTLSClientCertFingerprints, fp)
			}
		}

		contentJSON, err := json.Marshal(ocSeed.SystemSecurity)
		if err != nil {
			return err
		}

		_, err = doOCRequest(ctx, "http://localhost/1.0/system/security", http.MethodPut, contentJSON)
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

		_, err = doOCRequest(ctx, "http://localhost/1.0/system/updates", http.MethodPut, contentJSON)
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

// GetCertificate returns the keypair for the server certificate.
func (*operationsCenter) GetCertificate() (*tls.Certificate, error) {
	// Load the certificate.
	tlsCert, err := os.ReadFile("/var/lib/operations-center/server.crt")
	if err != nil {
		return nil, err
	}

	tlsKey, err := os.ReadFile("/var/lib/operations-center/server.key")
	if err != nil {
		return nil, err
	}

	// Put together a keypair.
	cert, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		return nil, err
	}

	return &cert, nil
}

// AddTrustedCertificate adds a new trusted certificate to the application.
func (*operationsCenter) AddTrustedCertificate(ctx context.Context, _ string, cert string) error {
	// Compute the certificate's fingerprint.
	fp, err := getCertificateFingerprint(cert)
	if err != nil {
		return err
	}

	// Get the current security configuration.
	body, err := doOCRequest(ctx, "http://localhost/1.0/system/security", http.MethodGet, nil)
	if err != nil {
		return err
	}

	sec := &api.SystemSecurity{}

	err = json.Unmarshal(body, sec)
	if err != nil {
		return err
	}

	// Check if the certificate is already trusted.
	if slices.Contains(sec.TrustedTLSClientCertFingerprints, fp) {
		return errors.New("client certificate is already trusted")
	}

	// Add the certificate's fingerprint to list of trusted clients.
	sec.TrustedTLSClientCertFingerprints = append(sec.TrustedTLSClientCertFingerprints, fp)

	contentJSON, err := json.Marshal(sec)
	if err != nil {
		return err
	}

	_, err = doOCRequest(ctx, "http://localhost/1.0/system/security", http.MethodPut, contentJSON)

	return err
}

// Operations Center specific helper to interact with the REST API.
func doOCRequest(ctx context.Context, url string, method string, body []byte) ([]byte, error) {
	return doRequest(ctx, "/run/operations-center/unix.socket", url, method, body)
}

// IsPrimary reports if the application is a primary application.
func (*operationsCenter) IsPrimary() bool {
	return true
}

// FactoryReset performs a full factory reset of the application.
func (oc *operationsCenter) FactoryReset(ctx context.Context) error {
	// Stop the application.
	err := oc.Stop(ctx, "")
	if err != nil {
		return err
	}

	// Wipe local configuration.
	err = oc.WipeLocalData()
	if err != nil {
		return err
	}

	// Start the application.
	err = oc.Start(ctx, "")
	if err != nil {
		return err
	}

	// Perform first start initialization.
	return oc.Initialize(ctx)
}

// WipeLocalData removes local data created by the application.
func (*operationsCenter) WipeLocalData() error {
	err := os.RemoveAll("/var/lib/operations-center/")
	if err != nil {
		return err
	}

	return os.Remove("/var/log/operations-center.log")
}

// GetBackup returns a tar archive backup of the application's configuration and/or state.
func (*operationsCenter) GetBackup(archive io.Writer, complete bool) error {
	if complete {
		return createTarArchive("/var/lib/operations-center/", nil, archive)
	}

	return createTarArchive("/var/lib/operations-center/", []string{"updates"}, archive)
}
