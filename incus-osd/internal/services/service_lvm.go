package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// LVM represents the system LVM service.
type LVM struct {
	state *state.State
}

// Get returns the current service state.
func (n *LVM) Get(ctx context.Context) (any, error) {
	// Get PV state.
	type pvsRaw struct {
		Report []struct {
			PV []api.ServiceLVMPV `json:"pv"`
		} `json:"report"`
	}

	pvsOutput, err := subprocess.RunCommandContext(ctx, "pvs", "--reportformat", "json_std")
	if err != nil {
		return nil, err
	}

	pvs := pvsRaw{}

	err = json.Unmarshal([]byte(pvsOutput), &pvs)
	if err != nil {
		return nil, err
	}

	if len(pvs.Report) > 0 {
		n.state.Services.LVM.State.PVs = pvs.Report[0].PV
	}

	// Get VG state.
	type vgsRaw struct {
		Report []struct {
			VG []api.ServiceLVMVG `json:"vg"`
		} `json:"report"`
		Log []api.ServiceLVMLog `json:"log"`
	}

	vgsOutput, err := subprocess.RunCommandContext(ctx, "vgs", "--reportformat", "json_std")
	if err != nil {
		return nil, err
	}

	vgs := vgsRaw{}

	err = json.Unmarshal([]byte(vgsOutput), &vgs)
	if err != nil {
		return nil, err
	}

	if len(vgs.Report) > 0 {
		n.state.Services.LVM.State.VGs = vgs.Report[0].VG
	}

	n.state.Services.LVM.State.Log = vgs.Log

	return n.state.Services.LVM, nil
}

// Update updates the service configuration.
func (n *LVM) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceLVM)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceLVM", req)
	}

	// Save the state on return.
	defer n.state.Save(ctx)

	// Disable the service if requested.
	if n.state.Services.LVM.Config.Enabled && !newState.Config.Enabled {
		err := n.Stop(ctx)
		if err != nil {
			return err
		}
	}

	// Enable the service if requested.
	if !n.state.Services.LVM.Config.Enabled && newState.Config.Enabled {
		// Update the configuration.
		n.state.Services.LVM.Config = newState.Config

		// Start the service.
		err := n.Start(ctx)
		if err != nil {
			return err
		}
	} else {
		// Update the configuration.
		n.state.Services.LVM.Config = newState.Config

		// Re-configure the service.
		err := n.configure(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the service.
func (n *LVM) Stop(ctx context.Context) error {
	if !n.state.Services.LVM.Config.Enabled {
		return nil
	}

	// Stop the lock manager.
	_, err := subprocess.RunCommandContext(ctx, "vgchange", "--lock-stop")
	if err != nil {
		return err
	}

	// Stop the units.
	err = systemd.StopUnit(ctx, "lvmlockd.service", "sanlock.service", "wdmd.service")
	if err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (n *LVM) Start(ctx context.Context) error {
	if !n.state.Services.LVM.Config.Enabled {
		return nil
	}

	// Generate configuration.
	err := n.configure(ctx)
	if err != nil {
		return err
	}

	// Start the units.
	err = systemd.StartUnit(ctx, "lvmlockd.service", "sanlock.service", "wdmd.service")
	if err != nil {
		return err
	}

	// Start the lock manager.
	_, err = subprocess.RunCommandContext(ctx, "vgchange", "--lock-start")
	if err != nil {
		return err
	}

	return nil
}

// ShouldStart returns true if the service should be started on boot.
func (n *LVM) ShouldStart() bool {
	return n.state.Services.LVM.Config.Enabled
}

// Struct returns the API struct for the LVM service.
func (*LVM) Struct() any {
	return &api.ServiceLVM{}
}

func (n *LVM) configure(_ context.Context) error {
	// Apply configuration.
	err := os.MkdirAll("/etc/lvm/", 0o700)
	if err != nil {
		return err
	}

	lvmlocal := fmt.Sprintf(`global {
	use_lvmlockd = 1
}

local {
	host_id = %d
}
`, n.state.Services.LVM.Config.SystemID)

	err = os.WriteFile("/etc/lvm/lvmlocal.conf", []byte(lvmlocal), 0o600)
	if err != nil {
		return err
	}

	return nil
}

func (*LVM) init(_ context.Context) error {
	return nil
}
