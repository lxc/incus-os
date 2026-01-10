package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// ISCSI represents the system ISCSI service.
type ISCSI struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *ISCSI) Get(_ context.Context) (any, error) {
	// Initialize target list if missing.
	if n.state.Services.ISCSI.Config.Targets == nil {
		n.state.Services.ISCSI.Config.Targets = []api.ServiceISCSITarget{}
	}

	// Get runtime details if enabled.
	if n.state.Services.ISCSI.Config.Enabled {
		// Retrieve host ID.
		initiatorName, err := os.ReadFile("/etc/iscsi/initiatorname.iscsi")
		if err != nil {
			return nil, err
		}

		n.state.Services.ISCSI.State.InitiatorName = strings.TrimPrefix(strings.TrimSpace(string(initiatorName)), "InitiatorName=")
	}

	return n.state.Services.ISCSI, nil
}

// Update updates the service configuration.
func (n *ISCSI) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceISCSI)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceISCSI", req)
	}

	// Save the state on return.
	defer n.state.Save()

	// Disable the service.
	err := n.Stop(ctx)
	if err != nil {
		return err
	}

	// Update the configuration.
	n.state.Services.ISCSI.Config = newState.Config

	// Bring the service back up.
	err = n.doStart(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Stop stops the service.
func (n *ISCSI) Stop(ctx context.Context) error {
	if !n.state.Services.ISCSI.Config.Enabled {
		return nil
	}

	// Disconnect from the targets.
	for _, target := range n.state.Services.ISCSI.Config.Targets {
		// Determine portal address.
		portal := target.Address
		if strings.Contains(portal, ":") {
			portal = "[" + portal + "]"
		}

		if target.Port > 0 {
			portal = fmt.Sprintf("%s:%d", portal, target.Port)
		}

		// Attempt logout from the target.
		_, _ = subprocess.RunCommandContext(ctx, "iscsiadm", "-m", "node", "-T", target.Target, "-p", portal, "--logout")
	}

	// Stop the systemd unit.
	err := systemd.StopUnit(ctx, "iscsid")
	if err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (n *ISCSI) Start(ctx context.Context) error {
	// Attempt to clear any leftover state.
	_ = os.RemoveAll("/var/lib/iscsi")

	return n.doStart(ctx)
}

// ShouldStart returns true if the service should be started on boot.
func (n *ISCSI) ShouldStart() bool {
	return n.state.Services.ISCSI.Config.Enabled
}

// Struct returns the API struct for the ISCSI service.
func (*ISCSI) Struct() any {
	return &api.ServiceISCSI{}
}

func (n *ISCSI) doStart(ctx context.Context) error {
	if !n.state.Services.ISCSI.Config.Enabled {
		return nil
	}

	// Create the ISCSI config directory if missing.
	err := os.Mkdir("/etc/iscsi", 0o700)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	// Create the host initiator name if missing.
	_, err = os.Stat("/etc/iscsi/initiatorname.iscsi")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		f, err := os.Create("/etc/iscsi/initiatorname.iscsi")
		if err != nil {
			return err
		}

		err = f.Chmod(0o600)
		if err != nil {
			return err
		}

		defer f.Close()

		output, err := subprocess.RunCommandContext(ctx, "iscsi-iname", "-p", "iqn.2004-10.org.linuxcontainers:01")
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(f, "InitiatorName=%s\n", strings.TrimSpace(output))
		if err != nil {
			return err
		}
	}

	// Generate iSCSI configuration if missing.
	_, err = os.Stat("/etc/iscsi/iscsid.conf")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		err = os.WriteFile("/etc/iscsi/iscsid.conf", []byte{}, 0o600)
		if err != nil {
			return err
		}
	}

	// Start the service.
	err = systemd.StartUnit(ctx, "iscsid")
	if err != nil {
		return err
	}

	// Connect to the targets.
	for _, target := range n.state.Services.ISCSI.Config.Targets {
		// Determine portal address.
		portal := target.Address
		if strings.Contains(portal, ":") {
			portal = "[" + portal + "]"
		}

		if target.Port > 0 {
			portal = fmt.Sprintf("%s:%d", portal, target.Port)
		}

		// Discover the targets.
		for range 10 {
			_, err = subprocess.RunCommandContext(ctx, "iscsiadm", "-m", "discovery", "-t", "sendtargets", "-p", portal)
			if err == nil {
				break
			}

			time.Sleep(500 * time.Millisecond)
		}

		if err != nil {
			return err
		}

		// Login to the target.
		_, err = subprocess.RunCommandContext(ctx, "iscsiadm", "-m", "node", "-T", target.Target, "-p", portal, "--login")
		if err != nil {
			return err
		}
	}

	return nil
}
