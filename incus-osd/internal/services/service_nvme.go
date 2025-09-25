package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// NVME represents the system NVME service.
type NVME struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *NVME) Get(_ context.Context) (any, error) {
	// Initialize target list if missing.
	if n.state.Services.NVME.Config.Targets == nil {
		n.state.Services.NVME.Config.Targets = []api.ServiceNVMETarget{}
	}

	// Get runtime details if enabled.
	if n.state.Services.NVME.Config.Enabled {
		// Retrieve host ID.
		hostid, err := os.ReadFile("/etc/nvme/hostid")
		if err != nil {
			return nil, err
		}

		n.state.Services.NVME.State.HostID = strings.TrimSpace(string(hostid))

		// Retrieve host NQN.
		hostnqn, err := os.ReadFile("/etc/nvme/hostnqn")
		if err != nil {
			return nil, err
		}

		n.state.Services.NVME.State.HostNQN = strings.TrimSpace(string(hostnqn))
	}

	return n.state.Services.NVME, nil
}

// Update updates the service configuration.
func (n *NVME) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceNVME)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceNVME", req)
	}

	// Save the state on return.
	defer n.state.Save()

	// Disable the service.
	err := n.Stop(ctx)
	if err != nil {
		return err
	}

	// Update the configuration.
	n.state.Services.NVME.Config = newState.Config

	// Bring the service back up.
	err = n.Start(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Stop stops the service.
func (n *NVME) Stop(ctx context.Context) error {
	if !n.state.Services.NVME.Config.Enabled {
		return nil
	}

	// Disconnect all the NVME devices.
	_, err := subprocess.RunCommandContext(ctx, "nvme", "disconnect-all")
	if err != nil {
		return err
	}

	// Remove the discovery file.
	err = os.Remove("/etc/nvme/discovery.conf")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

// Start starts the service.
func (n *NVME) Start(ctx context.Context) error {
	if !n.state.Services.NVME.Config.Enabled {
		return nil
	}

	// Ensure we have the right modules.
	for _, module := range []string{"nvme", "nvme-fabrics", "nvme-tcp"} {
		_, err := subprocess.RunCommandContext(ctx, "modprobe", module)
		if err != nil {
			return err
		}
	}

	// Create the NVME config directory if missing.
	err := os.Mkdir("/etc/nvme", 0o700)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	// Create the host NQN if missing.
	_, err = os.Stat("/etc/nvme/hostnqn")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		f, err := os.Create("/etc/nvme/hostnqn")
		if err != nil {
			return err
		}

		err = f.Chmod(0o600)
		if err != nil {
			return err
		}

		defer f.Close()

		err = subprocess.RunCommandWithFds(ctx, nil, f, "nvme", "gen-hostnqn")
		if err != nil {
			return err
		}
	}

	// Generate host ID if missing.
	_, err = os.Stat("/etc/nvme/hostid")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		err = os.WriteFile("/etc/nvme/hostid", append([]byte(uuid.New().String()), []byte("\n")...), 0o600)
		if err != nil {
			return err
		}
	}

	// Generate the targets.
	f, err := os.Create("/etc/nvme/discovery.conf")
	if err != nil {
		return err
	}

	defer f.Close()

	err = f.Chmod(0o600)
	if err != nil {
		return err
	}

	// Wait up to 30s for all targets to be contacted.
	ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, target := range n.state.Services.NVME.Config.Targets {
		// Attempt to connect to the target (wait up to 5s).
		//
		// This isn't fatal as some controllers may be temporarily offline.
		for range 10 {
			_, err = subprocess.RunCommandContext(ctxTimeout, "nvme", "discover", "--transport", target.Transport, "--traddr", target.Address, "--trsvcid", strconv.Itoa(target.Port))
			if err == nil {
				break
			}

			time.Sleep(500 * time.Millisecond)
		}

		_, err = fmt.Fprintf(f, "--transport=%s --traddr=%s --trsvcid=%d\n", target.Transport, target.Address, target.Port)
		if err != nil {
			return err
		}
	}

	// Wait up to 30s for all targets to be connected.
	ctxTimeout, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Connect all NVME devices.
	_, err = subprocess.RunCommandContext(ctxTimeout, "nvme", "connect-all")
	if err != nil {
		return err
	}

	return nil
}

// ShouldStart returns true if the service should be started on boot.
func (n *NVME) ShouldStart() bool {
	return n.state.Services.NVME.Config.Enabled
}

// Struct returns the API struct for the NVME service.
func (*NVME) Struct() any {
	return &api.ServiceNVME{}
}
