package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// USBIP represents the system USBIP service.
type USBIP struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *USBIP) Get(_ context.Context) (any, error) {
	// Initialize target list if missing.
	if n.state.Services.USBIP.Config.Targets == nil {
		n.state.Services.USBIP.Config.Targets = []api.ServiceUSBIPTarget{}
	}

	return n.state.Services.USBIP, nil
}

// Update updates the service configuration.
func (n *USBIP) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceUSBIP)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceUSBIP", req)
	}

	// Save the state on return.
	defer n.state.Save(ctx)

	// Update the configuration.
	n.state.Services.USBIP.Config = newState.Config

	// Attach the devices.
	err := n.Start(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (n *USBIP) Start(ctx context.Context) error {
	// If nothing to be attached, we're done.
	if len(n.state.Services.USBIP.Config.Targets) == 0 {
		return nil
	}

	// Load the kernel module.
	_, err := subprocess.RunCommandContext(ctx, "modprobe", "vhci-hcd")
	if err != nil {
		return err
	}

	// Attach all targets.
	for _, target := range n.state.Services.USBIP.Config.Targets {
		// Attempt to connect.
		_, err := subprocess.RunCommandContext(ctx, "usbip", "attach", "-r", target.Address, "-b", target.BusID)
		if err != nil {
			slog.WarnContext(ctx, "Unable to attach USBIP device", "address", target.Address, "busid", target.BusID, "err", err)
		}
	}

	return nil
}

// ShouldStart returns true if the service should be started on boot.
func (n *USBIP) ShouldStart() bool {
	return len(n.state.Services.USBIP.Config.Targets) > 0
}

// Struct returns the API struct for the USBIP service.
func (*USBIP) Struct() any {
	return &api.ServiceUSBIP{}
}
