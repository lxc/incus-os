package applications

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"time"

	ocapi "github.com/FuturFusion/operations-center/shared/api/system"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/zfs"
)

type operationsCenter struct {
	common
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

	sec := &ocapi.Security{}

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

// ConfigureLocalStorage configures local storage for the application.
func (oc *operationsCenter) ConfigureLocalStorage(ctx context.Context) error {
	// If the application isn't initialized, create a ZFS dataset for it to use.
	if !oc.IsInitialized() {
		err := zfs.CreateApplicationDataset(ctx, "operations-center")
		if err != nil {
			return err
		}
	} else {
		err := zfs.MountApplicationDataset(ctx, "operations-center")
		if err != nil {
			return err
		}
	}

	return nil
}

// FactoryReset performs a full factory reset of the application.
func (oc *operationsCenter) FactoryReset(ctx context.Context) error {
	// Stop the application.
	err := oc.Stop(ctx)
	if err != nil {
		return err
	}

	// Wipe local configuration.
	err = oc.WipeLocalData(ctx)
	if err != nil {
		return err
	}

	// Check if we're locally registered with it.
	if oc.state.System.Provider.State.Registered && (oc.state.System.Provider.Config.Config == nil || oc.state.System.Provider.Config.Config["server_url"] == "") {
		oc.state.System.Provider.State.Registered = false
		oc.state.System.Provider.Config.Config = map[string]string{}
	}

	// Start the application.
	err = oc.Start(ctx)
	if err != nil {
		return err
	}

	// Perform first start initialization.
	return oc.Initialize(ctx)
}

// GetBackup returns a tar archive backup of the application's configuration and/or state.
func (*operationsCenter) GetBackup(archive io.Writer, complete bool) error {
	if complete {
		return createTarArchive("/var/lib/operations-center/", nil, archive)
	}

	return createTarArchive("/var/lib/operations-center/", []string{"updates"}, archive)
}

// GetClientCertificate returns the keypair for the client certificate.
func (oc *operationsCenter) GetClientCertificate() (*tls.Certificate, error) {
	// Operations Center doesn't have a separate certificate it uses when contacting other servers.
	return oc.GetServerCertificate()
}

// GetDependencies returns a list of other applications this application depends on.
func (*operationsCenter) GetDependencies() []string {
	return nil
}

// GetServerCertificate returns the keypair for the server certificate.
func (*operationsCenter) GetServerCertificate() (*tls.Certificate, error) {
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

// Initialize runs first time initialization.
func (oc *operationsCenter) Initialize(ctx context.Context) error {
	// Get the preseed from the seed partition.
	ocSeed, err := seed.GetOperationsCenter(ctx)
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	// Configure an empty seed if none was provided.
	if ocSeed == nil {
		ocSeed = new(apiseed.OperationsCenter)
	}

	if ocSeed.Preseed == nil {
		ocSeed.Preseed = new(apiseed.OperationsCenterPreseed)
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
	if ocSeed.Preseed.SystemCertificate != nil {
		contentJSON, err := json.Marshal(ocSeed.Preseed.SystemCertificate)
		if err != nil {
			return err
		}

		_, err = doOCRequest(ctx, "http://localhost/1.0/system/certificate", http.MethodPost, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemNetwork, if any.
	if ocSeed.Preseed.SystemNetwork == nil {
		ocSeed.Preseed.SystemNetwork = new(ocapi.NetworkPut)
	}

	{
		// If no IP address is provided, default to listening on all addresses on port 8443.
		if ocSeed.Preseed.SystemNetwork.OperationsCenterAddress == "" && ocSeed.Preseed.SystemNetwork.RestServerAddress == "" {
			// Get the management address.
			mgmtAddr := oc.state.System.Network.State.GetInterfaceAddressByRole(api.SystemNetworkInterfaceRoleManagement)
			if mgmtAddr != nil {
				ocSeed.Preseed.SystemNetwork.OperationsCenterAddress = "https://" + net.JoinHostPort(mgmtAddr.String(), "8443")
			} else {
				ocSeed.Preseed.SystemNetwork.OperationsCenterAddress = "https://127.0.0.1:8443"
			}

			ocSeed.Preseed.SystemNetwork.RestServerAddress = "[::]:8443"
		}

		contentJSON, err := json.Marshal(ocSeed.Preseed.SystemNetwork)
		if err != nil {
			return err
		}

		_, err = doOCRequest(ctx, "http://localhost/1.0/system/network", http.MethodPut, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemSecurity, if any.
	if ocSeed.Preseed.SystemSecurity == nil && len(ocSeed.TrustedClientCertificates) > 0 {
		ocSeed.Preseed.SystemSecurity = new(ocapi.SecurityPut)
	}

	if ocSeed.Preseed.SystemSecurity != nil {
		// Compute fingerprints for any user-provided client certificates and add to the
		// list of trusted TLS client certificates.
		for i, certString := range ocSeed.TrustedClientCertificates {
			fp, err := getCertificateFingerprint(certString)
			if err != nil {
				return fmt.Errorf("%w (seed index %d)", err, i)
			}

			if !slices.Contains(ocSeed.Preseed.SystemSecurity.TrustedTLSClientCertFingerprints, fp) {
				ocSeed.Preseed.SystemSecurity.TrustedTLSClientCertFingerprints = append(ocSeed.Preseed.SystemSecurity.TrustedTLSClientCertFingerprints, fp)
			}
		}

		contentJSON, err := json.Marshal(ocSeed.Preseed.SystemSecurity)
		if err != nil {
			return err
		}

		_, err = doOCRequest(ctx, "http://localhost/1.0/system/security", http.MethodPut, contentJSON)
		if err != nil {
			return err
		}
	}

	// Apply SystemUpdates, if any.
	if ocSeed.Preseed.SystemUpdates != nil {
		contentJSON, err := json.Marshal(ocSeed.Preseed.SystemUpdates)
		if err != nil {
			return err
		}

		_, err = doOCRequest(ctx, "http://localhost/1.0/system/updates", http.MethodPut, contentJSON)
		if err != nil {
			return err
		}
	}

	oc.appState.Initialized = true

	return nil
}

// IsPrimary reports if the application is a primary application.
func (*operationsCenter) IsPrimary() bool {
	return true
}

// IsRunning reports if the application is currently running.
func (*operationsCenter) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "operations-center.service")
}

func (*operationsCenter) Name() string {
	return "operations-center"
}

// NeedsLateUpdateCheck reports if the application depends on a delayed provider update check.
func (*operationsCenter) NeedsLateUpdateCheck() bool {
	// Depends on application client TLS certificate stored in ZFS dataset.
	// Operations Center can also be self-hosted, which also requires a delay
	// before we can check for updates.
	return true
}

// Restart restarts the main systemd unit.
func (*operationsCenter) Restart(ctx context.Context) error {
	return systemd.RestartUnit(ctx, "operations-center.service")
}

// RestoreBackup restores a tar archive backup of the application's configuration and/or state.
func (oc *operationsCenter) RestoreBackup(ctx context.Context, archive io.Reader) error {
	err := extractTarArchive(ctx, "/var/lib/operations-center/", []string{"operations-center.service"}, archive)
	if err != nil {
		return err
	}

	// Record when the application was restored.
	now := time.Now()
	oc.appState.LastRestored = &now

	return nil
}

// Start starts the systemd unit.
func (oc *operationsCenter) Start(ctx context.Context) error {
	err := oc.ConfigureLocalStorage(ctx)
	if err != nil {
		return err
	}

	// Start the unit.
	return systemd.StartUnit(ctx, "operations-center.service")
}

// Stop stops the systemd unit.
func (*operationsCenter) Stop(ctx context.Context) error {
	// Stop the unit.
	return systemd.StopUnit(ctx, "operations-center.service")
}

// Update triggers restart after an application update.
func (*operationsCenter) Update(ctx context.Context) error {
	// Reload the systemd daemon to pickup any service definition changes.
	err := systemd.ReloadDaemon(ctx)
	if err != nil {
		return err
	}

	// Restart the unit.
	return systemd.RestartUnit(ctx, "operations-center.service")
}

// WipeLocalData removes local data created by the application.
func (*operationsCenter) WipeLocalData(ctx context.Context) error {
	// Unmount the dataset.
	err := unix.Unmount("/var/lib/operations-center/", 0)
	if err != nil {
		return err
	}

	// Destroy the dataset.
	err = zfs.DestroyDataset(ctx, "local", "operations-center", false)
	if err != nil {
		return err
	}

	// For good measure, remove the mount point.
	err = os.RemoveAll("/var/lib/operations-center/")
	if err != nil {
		return err
	}

	// Create a fresh dataset for the application.
	return zfs.CreateApplicationDataset(ctx, "operations-center")
}

// Operations Center specific helper to interact with the REST API.
func doOCRequest(ctx context.Context, url string, method string, body []byte) ([]byte, error) {
	return doRequest(ctx, "/run/operations-center/unix.socket", url, method, body)
}

func (oc *operationsCenter) Get(_ context.Context) (any, error) {
	return oc.state.Applications.OperationsCenter, nil
}

func (*operationsCenter) Struct() any {
	return &api.Application{}
}

func (*operationsCenter) UpdateConfig(_ context.Context, _ any) error {
	return nil
}
