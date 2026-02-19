package applications

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"os"

	incusclient "github.com/lxc/incus/v6/client"
	incusapi "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/subprocess"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type incusDebug struct {
	Action string `json:"action"`
}

type incus struct {
	common
}

func (*incus) Name() string {
	return "incus"
}

type incusCeph struct {
	common
}

func (*incusCeph) Name() string {
	return "incus-ceph"
}

// GetDependencies returns a list of other applications this application depends on.
func (*incusCeph) GetDependencies() []string {
	return []string{"incus"}
}

type incusLinstor struct {
	common
}

func (*incusLinstor) Name() string {
	return "incus-linstor"
}

// GetDependencies returns a list of other applications this application depends on.
func (*incusLinstor) GetDependencies() []string {
	return []string{"incus"}
}

// Debug runs a debug action.
func (*incus) Debug(ctx context.Context, data any) response.Response {
	req, ok := data.(*incusDebug)
	if !ok {
		return response.BadRequest(errors.New("invalid request data type"))
	}

	switch req.Action {
	case "get-logs":
		pr, pw := io.Pipe()

		go func() {
			_ = subprocess.RunCommandWithFds(ctx, nil, pw, "tar", "-zcf", "-", "/run/incus/", "/var/log/incus/")
			_ = pw.Close()
		}()

		return response.PipeResponse(pr)

	default:
		return response.NotImplemented(nil)
	}
}

// DebugStruct returns the struct to fill with debug request data.
func (*incus) DebugStruct() any {
	data := &incusDebug{}

	return data
}

// Start starts all the systemd units.
func (*incus) Start(ctx context.Context, _ string) error {
	// Refresh the system users.
	err := systemd.RefreshUsers(ctx)
	if err != nil {
		return err
	}

	// Refresh the sysctls.
	err = systemd.RestartUnit(ctx, "systemd-sysctl.service")
	if err != nil {
		return err
	}

	// Start the units.
	return systemd.EnableUnit(ctx, true, "incus.socket", "incus-lxcfs.service", "incus-startup.service", "incus.service")
}

// Stop stops all the systemd units.
func (*incus) Stop(ctx context.Context, _ string) error {
	// Trigger a clean shutdown.
	err := systemd.StopUnit(ctx, "incus-startup.service")
	if err != nil {
		return err
	}

	// Stop the remaining units.
	err = systemd.StopUnit(ctx, "incus.service", "incus.socket", "incus-lxcfs.service")
	if err != nil {
		return err
	}

	return nil
}

// Restart restarts the main systemd unit.
func (*incus) Restart(ctx context.Context, _ string) error {
	return systemd.RestartUnit(ctx, "incus.service")
}

// Update triggers a partial restart after an application update.
func (*incus) Update(ctx context.Context, _ string) error {
	// Refresh the system users.
	err := systemd.RefreshUsers(ctx)
	if err != nil {
		return err
	}

	// Reload the systemd daemon to pickup any service definition changes.
	err = systemd.ReloadDaemon(ctx)
	if err != nil {
		return err
	}

	// Restart the main unit.
	return systemd.RestartUnit(ctx, "incus.service")
}

// Initialize runs first time initialization.
func (a *incus) Initialize(ctx context.Context) error {
	// Get the preseed from the seed partition.
	incusSeed, err := seed.GetIncus(ctx)
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	// If no seed, build one for auto-configuration.
	if incusSeed == nil {
		incusSeed = &apiseed.Incus{
			ApplyDefaults: true,
		}
	}

	// Connect to Incus.
	c, err := incusclient.ConnectIncusUnix("", nil)
	if err != nil {
		return err
	}

	// Push the preseed if one is present.
	if incusSeed.Preseed != nil {
		err = c.ApplyServerPreseed(*incusSeed.Preseed)
		if err != nil {
			return err
		}
	}

	// Handle the defaults.
	if incusSeed.ApplyDefaults {
		err = a.applyDefaults(ctx, c)
		if err != nil {
			return err
		}
	}

	// Set listen address if not set.
	conf, etag, err := c.GetServer()
	if err != nil {
		return err
	}

	_, ok := conf.Config["core.https_address"]
	if !ok {
		conf.Config["core.https_address"] = ":8443"

		err = c.UpdateServer(conf.Writable(), etag)
		if err != nil {
			return err
		}
	}

	return nil
}

// IsRunning reports if the application is currently running.
func (*incus) IsRunning(ctx context.Context) bool {
	return systemd.IsActive(ctx, "incus.service")
}

// NeedsLateUpdateCheck reports if the application depends on a delayed provider update check.
func (*incus) NeedsLateUpdateCheck() bool {
	return false
}

// IsPrimary reports if the application is a primary application.
func (*incus) IsPrimary() bool {
	return true
}

// GetClientCertificate returns the keypair for the client certificate.
func (a *incus) GetClientCertificate() (*tls.Certificate, error) {
	return a.getCertificate("server")
}

// GetServerCertificate returns the keypair for the server certificate.
func (a *incus) GetServerCertificate() (*tls.Certificate, error) {
	cert, err := a.getCertificate("cluster")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		return a.getCertificate("server")
	}

	return cert, nil
}

// GetDependencies returns a list of other applications this application depends on.
func (*incus) GetDependencies() []string {
	return nil
}

// AddTrustedCertificate adds a new trusted certificate to the application.
func (*incus) AddTrustedCertificate(_ context.Context, name string, cert string) error {
	// Connect to Incus.
	c, err := incusclient.ConnectIncusUnix("", nil)
	if err != nil {
		return err
	}

	// Add the certificate.
	req := incusapi.CertificatesPost{}
	req.Name = name
	req.Type = "client"
	req.Certificate = cert

	err = c.CreateCertificate(req)
	if err != nil {
		return err
	}

	return nil
}

// FactoryReset performs a full factory reset of the application.
func (a *incus) FactoryReset(ctx context.Context) error {
	// Stop the application.
	err := a.Stop(ctx, "")
	if err != nil {
		return err
	}

	// Wipe local configuration.
	err = a.WipeLocalData()
	if err != nil {
		return err
	}

	// Start the application.
	err = a.Start(ctx, "")
	if err != nil {
		return err
	}

	// Perform first start initialization.
	return a.Initialize(ctx)
}

// WipeLocalData removes local data created by the application.
func (*incus) WipeLocalData() error {
	err := os.RemoveAll("/var/lib/incus/")
	if err != nil {
		return err
	}

	err = os.RemoveAll("/var/lib/incus-lxcfs/")
	if err != nil {
		return err
	}

	return os.RemoveAll("/var/log/incus/")
}

// GetBackup returns a tar archive backup of the application's configuration and/or state.
func (*incus) GetBackup(archive io.Writer, _ bool) error {
	return createTarArchive("/var/lib/incus/", []string{"guestapi", "shmounts", "unix.socket"}, archive)
}

// RestoreBackup restores a tar archive backup of the application's configuration and/or state.
func (*incus) RestoreBackup(ctx context.Context, archive io.Reader) error {
	return extractTarArchive(ctx, "/var/lib/incus/", []string{"incus-startup.service", "incus.socket", "incus.service", "incus-lxcfs.service"}, archive)
}

func (*incus) applyDefaults(ctx context.Context, c incusclient.InstanceServer) error {
	// Get server configuration.
	serverConfig, serverConfigEtag, err := c.GetServer()
	if err != nil {
		return err
	}

	// Get the default profile.
	profileDefault, profileDefaultEtag, err := c.GetProfile("default")
	if err != nil {
		return err
	}

	if profileDefault.Devices == nil {
		profileDefault.Devices = map[string]map[string]string{}
	}

	// Check for storage pools.
	storagePools, err := c.GetStoragePoolNames()
	if err != nil {
		return err
	}

	// Check for networks.
	allNetworks, err := c.GetNetworks()
	if err != nil {
		return err
	}

	networks := []incusapi.Network{}

	for _, network := range allNetworks {
		if !network.Managed {
			continue
		}

		networks = append(networks, network)
	}

	// Create storage pools.
	if len(storagePools) == 0 && storage.PoolExists(ctx, "local") && !storage.DatasetExists(ctx, "local/incus") {
		// Create the local pool.
		err = c.CreateStoragePool(incusapi.StoragePoolsPost{
			Name:   "local",
			Driver: "zfs",
			StoragePoolPut: incusapi.StoragePoolPut{
				Config: map[string]string{
					"source": "local/incus",
				},
				Description: "Local storage pool (on system drive)",
			},
		})
		if err != nil {
			return err
		}

		// Set the incusos:use property on the newly created pool.
		_, err = subprocess.RunCommand("zfs", "set", "incusos:use=incus", "local/incus")
		if err != nil {
			return err
		}

		// Create the default volumes.
		for _, volName := range []string{"backups", "images"} {
			// Create the volume.
			err = c.CreateStoragePoolVolume("local", incusapi.StorageVolumesPost{
				Name:        volName,
				Type:        "custom",
				ContentType: "filesystem",
				StorageVolumePut: incusapi.StorageVolumePut{
					Description: "Volume holding system " + volName,
				},
			})
			if err != nil {
				return err
			}

			// Make use of it.
			serverConfig.Config[fmt.Sprintf("storage.%s_volume", volName)] = "local/" + volName
		}

		// Add to the default profile.
		profileDefault.Devices["root"] = map[string]string{
			"type": "disk",
			"path": "/",
			"pool": "local",
		}
	}

	// Create networks.
	if len(networks) == 0 {
		// Create the incusbr0 network.
		err = c.CreateNetwork(incusapi.NetworksPost{
			Name: "incusbr0",
			NetworkPut: incusapi.NetworkPut{
				Description: "Local network bridge (NAT)",
			},
		})
		if err != nil {
			return err
		}

		// Add to the default profile.
		profileDefault.Devices["eth0"] = map[string]string{
			"type":    "nic",
			"network": "incusbr0",
			"name":    "eth0",
		}
	}

	// Listen on the network by default.
	_, ok := serverConfig.Config["core.https_address"]
	if !ok {
		serverConfig.Config["core.https_address"] = ":8443"
	}

	// Apply default profile changes.
	err = c.UpdateProfile("default", profileDefault.Writable(), profileDefaultEtag)
	if err != nil {
		return err
	}

	// Apply server configuration.
	err = c.UpdateServer(serverConfig.Writable(), serverConfigEtag)
	if err != nil {
		return err
	}

	return nil
}

func (*incus) getCertificate(name string) (*tls.Certificate, error) {
	// Load the certificate.
	tlsCert, err := os.ReadFile("/var/lib/incus/" + name + ".crt")
	if err != nil {
		return nil, err
	}

	tlsKey, err := os.ReadFile("/var/lib/incus/" + name + ".key")
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
