package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Ceph represents the system Ceph service.
type Ceph struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *Ceph) Get(_ context.Context) (any, error) {
	// Initialize target list if missing.
	if n.state.Services.Ceph.Config.Clusters == nil {
		n.state.Services.Ceph.Config.Clusters = map[string]api.ServiceCephCluster{}
	}

	return n.state.Services.Ceph, nil
}

// Update updates the service configuration.
func (n *Ceph) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceCeph)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceCeph", req)
	}

	// Save the state on return.
	defer n.state.Save()

	// Disable the service.
	err := n.Stop(ctx)
	if err != nil {
		return err
	}

	// Update the configuration.
	n.state.Services.Ceph.Config = newState.Config

	// Bring the service back up.
	err = n.Start(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Stop stops the service.
func (n *Ceph) Stop(_ context.Context) error {
	if !n.state.Services.Ceph.Config.Enabled {
		return nil
	}

	// Remove Ceph config.
	err := os.RemoveAll("/etc/ceph")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

// Start starts the service.
func (n *Ceph) Start(_ context.Context) error {
	if !n.state.Services.Ceph.Config.Enabled {
		return nil
	}

	// Create the Ceph config directory if missing.
	err := os.Mkdir("/etc/ceph", 0o755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	writeConfig := func(clusterName string, cluster api.ServiceCephCluster) error {
		// Generate the main configuration file.
		rw, err := os.Create(filepath.Join("/etc/ceph", clusterName+".conf")) //nolint:gosec
		if err != nil {
			return err
		}

		defer rw.Close()

		err = rw.Chmod(0o644)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(rw, `[global]
fsid = %s
mon_host = %s
`, cluster.FSID, strings.Join(cluster.Monitors, ","))
		if err != nil {
			return err
		}

		if len(cluster.ClientConfig) > 0 {
			_, err = fmt.Fprint(rw, "\n[client]\n")
			if err != nil {
				return err
			}

			for k, v := range cluster.ClientConfig {
				_, err = fmt.Fprintf(rw, "%s = %s\n", k, v)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}

	writeKeyring := func(clusterName string, keyringName string, keyring api.ServiceCephKeyring) error {
		// Generate the keyring file.
		rw, err := os.Create(filepath.Join("/etc/ceph", clusterName+".client."+keyringName+".keyring")) //nolint:gosec
		if err != nil {
			return err
		}

		err = rw.Chmod(0o600)
		if err != nil {
			return err
		}

		defer rw.Close()

		_, err = fmt.Fprintf(rw, `[client.%s]
key = %s
`, keyringName, keyring.Key)
		if err != nil {
			return err
		}

		return nil
	}

	// Generate the targets and keyrings.
	for clusterName, cluster := range n.state.Services.Ceph.Config.Clusters {
		err := writeConfig(clusterName, cluster)
		if err != nil {
			return err
		}

		for keyringName, keyring := range cluster.Keyrings {
			err := writeKeyring(clusterName, keyringName, keyring)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ShouldStart returns true if the service should be started on boot.
func (n *Ceph) ShouldStart() bool {
	return n.state.Services.Ceph.Config.Enabled
}

// Struct returns the API struct for the Ceph service.
func (*Ceph) Struct() any {
	return &api.ServiceCeph{}
}

// Supported returns whether the system can use Ceph.
func (n *Ceph) Supported() bool {
	// Ceph requires incus-ceph to be installed.
	_, ok := n.state.Applications["incus-ceph"]

	return ok
}
