package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// Netbird represents the system Netbird service.
type Netbird struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *Netbird) Get(_ context.Context) (any, error) {
	return n.state.Services.Netbird, nil
}

// Update updates the service configuration.
func (n *Netbird) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceNetbird)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceNetbird", req)
	}

	// Save the state on return.
	defer n.state.Save()

	// Disable the service if requested.
	if n.state.Services.Netbird.Config.Enabled && !newState.Config.Enabled {
		// Stop the service.
		err := n.Stop(ctx)
		if err != nil {
			return err
		}
	}

	// Apply the new configuration.
	n.state.Services.Netbird.Config = newState.Config

	if n.state.Services.Netbird.Config.Enabled {
		// Ensure the service is running.
		err := n.Start(ctx)
		if err != nil {
			return err
		}

		// Apply the configuration.
		err = n.configure(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the service.
func (n *Netbird) Stop(ctx context.Context) error {
	if !n.state.Services.Netbird.Config.Enabled {
		return nil
	}

	// Stop the multipath service.
	err := systemd.StopUnit(ctx, "netbird.service")
	if err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (n *Netbird) Start(ctx context.Context) error {
	if !n.state.Services.Netbird.Config.Enabled {
		return nil
	}

	// Ensure the service is running.
	err := systemd.StartUnit(ctx, "netbird.service")
	if err != nil {
		return err
	}

	return nil
}

// ShouldStart returns true if the service should be started on boot.
func (n *Netbird) ShouldStart() bool {
	return n.state.Services.Netbird.Config.Enabled
}

// Struct returns the API struct for the Netbird service.
func (*Netbird) Struct() any {
	return &api.ServiceNetbird{}
}

// configure applies the Netbird configuration
func (n *Netbird) configure(ctx context.Context) error {
	// Logout of any existing environment.
	_, err := subprocess.RunCommandContext(ctx, "netbird", "down")
	if err != nil {
		return err
	}

	// Join with the provided key, management and admin server.
	_, err = subprocess.RunCommandContext(
		ctx,
		"netbird",
		"login",
		"--no-browser",
		"--setup-key",
		n.state.Services.Netbird.Config.SetupKey,
		"--management-url",
		n.state.Services.Netbird.Config.ManagementUrl,
		"--admin-url",
		n.state.Services.Netbird.Config.AdminUrl,
	)

	// Connect to netbird with the supplied configuration.
	args := []string{
		"up",
		"--no-browser",
		"--dns-resolver-address",
		n.state.Services.Netbird.Config.DnsResolverAddress,
		"--external-ip-map",
		strings.Join(n.state.Services.Netbird.Config.ExternalIpMap, ","),
		"--extra-dns-labels",
		strings.Join(n.state.Services.Netbird.Config.ExtraDnsLabels, ","),
	}
	if n.state.Services.Netbird.Config.Anonymize {
		args = append(args, "--anonymize")
	}
	if n.state.Services.Netbird.Config.BlockInbound {
		args = append(args, "--block-inbound")
	}
	if n.state.Services.Netbird.Config.BlockLanAccess {
		args = append(args, "--block-lan-access")
	}
	if n.state.Services.Netbird.Config.DisableClientRoutes {
		args = append(args, "--disable-client-routes")
	}
	if n.state.Services.Netbird.Config.DisableServerRoutes {
		args = append(args, "--disable-server-routes")
	}
	if n.state.Services.Netbird.Config.DisableDns {
		args = append(args, "--disable-dns")
	}
	if n.state.Services.Netbird.Config.DisableFirewall {
		args = append(args, "--disable-firewall")
	}

	_, err = subprocess.RunCommandContext(ctx, "netbird", args...)
	if err != nil {
		return err
	}

	return nil
}
