package rest

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Server holds the internal state of the REST API server.
type Server struct {
	socketPath string
	state      *state.State
}

// NewServer returns a REST API server object.
func NewServer(_ context.Context, s *state.State, socketPath string) (*Server, error) {
	// Define the struct.
	server := Server{
		socketPath: socketPath,
		state:      s,
	}

	// Create runtime path if missing.
	err := os.Mkdir(filepath.Dir(socketPath), 0o700)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	return &server, nil
}

// Serve starts the REST API server.
func (s *Server) Serve(ctx context.Context) error {
	// Setup listener.
	_ = os.Remove(s.socketPath)
	lc := &net.ListenConfig{}

	listener, err := lc.Listen(ctx, "unix", s.socketPath)
	if err != nil {
		return err
	}

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
	router.HandleFunc("/1.0/applications/{name}/:factory-reset", s.apiApplicationsFactoryReset)
	router.HandleFunc("/1.0/applications/{name}/:restart", s.apiApplicationsRestart)
	router.HandleFunc("/1.0/applications/{name}/:restore", s.apiApplicationsRestore)
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
	router.HandleFunc("/1.0/system/kernel", s.apiSystemKernel)
	router.HandleFunc("/1.0/system/logging", s.apiSystemLogging)
	router.HandleFunc("/1.0/system/network", s.apiSystemNetwork)
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
		Handler: router,

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
	}

	return server.Serve(listener)
}
