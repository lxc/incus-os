package applications

import (
	"context"
	"fmt"
	"os"

	incusclient "github.com/lxc/incus/v6/client"
	incusapi "github.com/lxc/incus/v6/shared/api"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type incus struct{}

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
	err = systemd.StopUnit(ctx, "incus.service", "incus-lxcfs.service")
	if err != nil {
		return err
	}

	return nil
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
	incusSeed, err := seed.GetIncus(ctx, seed.GetSeedPath())
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
		err = a.applyDefaults(c)
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

// Uninstall uninstalls the application, and optionally removes any local user data.
func (a *incus) Uninstall(ctx context.Context, removeUserData bool) error {
	// TODO: Stop all containers/VMs.

	if removeUserData {
		// TODO: Destroy any local data pools.
	}

	// Stop the application.
	err := a.Stop(ctx, "")
	if err != nil {
		return err
	}

	// Remove the sysext image.
	err = os.Remove("/var/lib/extensions/incus.raw")
	if err != nil {
		return err
	}

	// Refresh the system extensions.
	err = systemd.RefreshExtensions(ctx)
	if err != nil {
		return err
	}

	// Reload the systemd daemon.
	err = systemd.ReloadDaemon(ctx)
	if err != nil {
		return err
	}

	if removeUserData {
		err := os.RemoveAll("/var/lib/incus/")
		if err != nil {
			return err
		}
	}

	return nil
}

func (*incus) applyDefaults(c incusclient.InstanceServer) error {
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
	if len(storagePools) == 0 {
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
