package rest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Server holds the internal state of the REST API server.
type Server struct {
	listener net.Listener
	state    *state.State
}

// NewServer returns a REST API server object.
func NewServer(_ context.Context, s *state.State, l net.Listener) (*Server, error) {
	// Define the struct.
	server := Server{
		listener: l,
		state:    s,
	}

	return &server, nil
}

// Serve starts the REST API server.
func (s *Server) Serve() error {
	// Setup routing.
	router := http.NewServeMux()

	router.HandleFunc("/", s.apiRoot)
	router.HandleFunc("/internal/auth/:generate-registration", s.apiInternalRegistration)
	router.HandleFunc("/internal/auth/:generate-token", s.apiInternalToken)
	router.HandleFunc("/internal/tui/:write-message", s.apiInternalTUI)
	router.HandleFunc("/1.0", s.apiRoot10)
	router.HandleFunc("/1.0/applications", s.apiApplications)
	router.HandleFunc("/1.0/applications/{name}", s.apiApplicationsEndpoint)
	router.HandleFunc("/1.0/applications/{name}/:backup", s.apiApplicationsBackup)
	router.HandleFunc("/1.0/applications/{name}/:check-update", s.apiApplicationsCheckUpdate)
	router.HandleFunc("/1.0/applications/{name}/:debug", s.apiApplicationsDebug)
	router.HandleFunc("/1.0/applications/{name}/:factory-reset", s.apiApplicationsFactoryReset)
	router.HandleFunc("/1.0/applications/{name}/:remove", s.apiApplicationsRemove)
	router.HandleFunc("/1.0/applications/{name}/:restart", s.apiApplicationsRestart)
	router.HandleFunc("/1.0/applications/{name}/:restore", s.apiApplicationsRestore)
	router.HandleFunc("/1.0/applications/{name}/:switch-version", s.apiApplicationsSwitchVersion)
	router.HandleFunc("/1.0/debug", s.apiDebug)
	router.HandleFunc("/1.0/debug/log", s.apiDebugLog)
	router.HandleFunc("/1.0/debug/processes", s.apiDebugProcesses)
	router.HandleFunc("/1.0/debug/secureboot", s.apiDebugSecureBoot)
	router.HandleFunc("/1.0/debug/secureboot/event-log", s.apiDebugSecureBootEventLog)
	router.HandleFunc("/1.0/debug/secureboot/:update", s.apiDebugSecureBootUpdate)
	router.HandleFunc("/1.0/services", s.apiServices)
	router.HandleFunc("/1.0/services/{name}", s.apiServicesEndpoint)
	router.HandleFunc("/1.0/services/{name}/:reset", s.apiServicesEndpointReset)
	router.HandleFunc("/1.0/system", s.apiSystem)
	router.HandleFunc("/1.0/system/:backup", s.apiSystemBackup)
	router.HandleFunc("/1.0/system/:factory-reset", s.apiSystemFactoryReset)
	router.HandleFunc("/1.0/system/:poweroff", s.apiSystemPoweroff)
	router.HandleFunc("/1.0/system/:reboot", s.apiSystemReboot)
	router.HandleFunc("/1.0/system/:restore", s.apiSystemRestore)
	router.HandleFunc("/1.0/system/:suspend", s.apiSystemSuspend)
	router.HandleFunc("/1.0/system/kernel", s.apiSystemKernel)
	router.HandleFunc("/1.0/system/logging", s.apiSystemLogging)
	router.HandleFunc("/1.0/system/network", s.apiSystemNetwork)
	router.HandleFunc("/1.0/system/network/:confirm", s.apiSystemNetworkConfirm)
	router.HandleFunc("/1.0/system/network/:flush-dns", s.apiSystemNetworkFlushDNS)
	router.HandleFunc("/1.0/system/provider", s.apiSystemProvider)
	router.HandleFunc("/1.0/system/resources", s.apiSystemResources)
	router.HandleFunc("/1.0/system/security", s.apiSystemSecurity)
	router.HandleFunc("/1.0/system/security/:tpm-rebind", s.apiSystemSecurityTPMRebind)
	router.HandleFunc("/1.0/system/storage", s.apiSystemStorage)
	router.HandleFunc("/1.0/system/storage/:create-volume", s.apiSystemStorageCreateVolume)
	router.HandleFunc("/1.0/system/storage/:import-encrypted-drive", s.apiSystemStorageImportEncryptedDrive)
	router.HandleFunc("/1.0/system/storage/:encrypt-drive", s.apiSystemStorageEncryptDrive)
	router.HandleFunc("/1.0/system/storage/:delete-pool", s.apiSystemStorageDeletePool)
	router.HandleFunc("/1.0/system/storage/:delete-volume", s.apiSystemStorageDeleteVolume)
	router.HandleFunc("/1.0/system/storage/:import-pool", s.apiSystemStorageImportPool)
	router.HandleFunc("/1.0/system/storage/:wipe-drive", s.apiSystemStorageWipeDrive)
	router.HandleFunc("/1.0/system/storage/:scrub-pool", s.apiSystemStorageScrubPool)
	router.HandleFunc("/1.0/system/update", s.apiSystemUpdate)
	router.HandleFunc("/1.0/system/update/:check", s.apiSystemUpdateCheck)

	// Setup server.
	server := &http.Server{
		// If not listening on the local Unix socket, define a custom handler that first checks for
		// a trusted client TLS certificate and then ensures a proper proxy header is present and
		// trims the "/os" prefix if present before passing the request to the standard handler.
		Handler: func(h http.Handler) http.Handler {
			if s.listener.Addr().Network() != "unix" {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.TLS == nil {
						http.Error(w, "Upgrade Required", http.StatusUpgradeRequired)

						return
					}

					if len(r.TLS.PeerCertificates) == 0 {
						http.Error(w, "Forbidden", http.StatusForbidden)

						return
					}

					// Verify the client provided a TLS certificate that matches a trusted fingerprint.
					foundTrustedClient := false

					clientFp := sha256.Sum256(r.TLS.PeerCertificates[0].Raw)

					for _, trustedCert := range s.state.System.FallbackListener.Config.TrustedClientCertificates {
						block, _ := pem.Decode([]byte(trustedCert))
						if block == nil || block.Type != "CERTIFICATE" {
							continue
						}

						cert, err := x509.ParseCertificate(block.Bytes)
						if err != nil {
							continue
						}

						certFp := sha256.Sum256(cert.Raw)

						if bytes.Equal(clientFp[:], certFp[:]) {
							foundTrustedClient = true

							break
						}
					}

					if !foundTrustedClient {
						http.Error(w, "Forbidden", http.StatusForbidden)

						return
					}

					// Ensure a proper proxy header is set and trim the "/os" prefix if present.
					r.Header.Set("X-IncusOS-Proxy", "/os")
					r.URL.Path = strings.TrimPrefix(r.URL.Path, "/os")

					// Serve the request.
					h.ServeHTTP(w, r)
				})
			}

			return h
		}(router),

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
	}

	return server.Serve(s.listener)
}
