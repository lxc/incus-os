package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// Tailscale represents the system Tailscale service.
type Tailscale struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *Tailscale) Get(_ context.Context) (any, error) {
	return n.state.Services.Tailscale, nil
}

// Update updates the service configuration.
func (n *Tailscale) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceTailscale)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceTailscale", req)
	}

	// Save the state on return.
	defer n.state.Save()

	// Disable the service if requested.
	if n.state.Services.Tailscale.Config.Enabled && !newState.Config.Enabled {
		// Stop the service.
		err := n.Stop(ctx)
		if err != nil {
			return err
		}

		// Apply the new configuration.
		n.state.Services.Tailscale.Config = newState.Config
	} else {
		// Check for config changes.
		needsRejoin := newState.Config.LoginServer != n.state.Services.Tailscale.Config.LoginServer || newState.Config.AuthKey != n.state.Services.Tailscale.Config.AuthKey

		// Apply the new configuration.
		n.state.Services.Tailscale.Config = newState.Config

		// Ensure the service is running.
		err := n.Start(ctx)
		if err != nil {
			return err
		}

		// Apply the configuration.
		err = n.configure(ctx, needsRejoin)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the service.
func (n *Tailscale) Stop(ctx context.Context) error {
	if !n.state.Services.Tailscale.Config.Enabled {
		return nil
	}

	// Stop the multipath service.
	err := systemd.StopUnit(ctx, "tailscale.service")
	if err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (n *Tailscale) Start(ctx context.Context) error {
	if !n.state.Services.Tailscale.Config.Enabled {
		return nil
	}

	// Ensure the service is running.
	err := systemd.StartUnit(ctx, "tailscale.service")
	if err != nil {
		return err
	}

	return nil
}

// ShouldStart returns true if the service should be started on boot.
func (n *Tailscale) ShouldStart() bool {
	return n.state.Services.Tailscale.Config.Enabled
}

// Struct returns the API struct for the Tailscale service.
func (*Tailscale) Struct() any {
	return &api.ServiceTailscale{}
}

// configure applies the Tailscale configuration.
func (n *Tailscale) configure(ctx context.Context, needsRejoin bool) error {
	if needsRejoin {
		// Logout of any existing environment.
		_, err := subprocess.RunCommandContext(ctx, "tailscale", "down")
		if err != nil {
			return err
		}

		// Join with the provided key and login server.
		args := []string{"up", "--auth-key", n.state.Services.Tailscale.Config.AuthKey}
		if n.state.Services.Tailscale.Config.LoginServer != "" {
			args = append(args, "--login-server", n.state.Services.Tailscale.Config.LoginServer)
		}

		_, err = subprocess.RunCommandContext(ctx, "tailscale", args...)
		if err != nil {
			return err
		}
	}

	_, err := subprocess.RunCommandContext(ctx, "tailscale", "set", "--advertise-routes", strings.Join(n.state.Services.Tailscale.Config.AdvertisedRoutes, ","), "--accept-routes="+strconv.FormatBool(n.state.Services.Tailscale.Config.AcceptRoutes))
	if err != nil {
		return err
	}

	return nil
}
