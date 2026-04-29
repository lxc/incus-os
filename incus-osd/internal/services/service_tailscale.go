package services

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
	"tailscale.com/ipn/ipnstate"

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
func (n *Tailscale) Get(ctx context.Context) (any, error) {
	// Set timeout in case tailscale status is unresponsive.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get the current Tailscale status through tailscale status --json command.
	tailscaleStatusJSONOutput, err := subprocess.RunCommandContext(ctx, "tailscale", "status", "--json")
	if err != nil {
		// If Tailscale isn't running, return a basic struct with the current configuration.
		if strings.Contains(err.Error(), "Failed to connect to local Tailscale daemon") {
			return api.ServiceTailscale{
				Config: n.state.Services.Tailscale.Config,
			}, nil
		}

		return nil, fmt.Errorf("failed to run tailscale status: %w", err)
	}

	// Parse the JSON output into a ServiceTailscaleState struct.
	parsedState, err := parseTailscaleStatusJSON([]byte(tailscaleStatusJSONOutput))
	if err != nil {
		return nil, fmt.Errorf("failed to parse tailscale status JSON: %w", err)
	}

	// Construct the ServiceTailscale struct with the current configuration and state.
	res := api.ServiceTailscale{
		Config: n.state.Services.Tailscale.Config,
		State:  *parsedState,
	}

	return res, nil
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
		needsRejoin := func(oldConfig api.ServiceTailscaleConfig, newConfig api.ServiceTailscaleConfig) bool {
			if oldConfig.Enabled != newConfig.Enabled {
				return true
			}

			if oldConfig.LoginServer != newConfig.LoginServer {
				return true
			}

			if oldConfig.AuthKey != newConfig.AuthKey {
				return true
			}

			return false
		}(n.state.Services.Tailscale.Config, newState.Config)

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
		args := []string{"up", "--reset", "--auth-key", n.state.Services.Tailscale.Config.AuthKey}
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

	_, err = subprocess.RunCommandContext(ctx, "tailscale", "serve", "reset")
	if err != nil {
		return err
	}

	if n.state.Services.Tailscale.Config.ServeEnabled {
		// Timeout since there this command is interactive when the tailnet admin has not provisioned HTTPS
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Prepare argument list, which gets appended to in the case serve_service is set to non-empty.
		args := []string{
			"serve",
			"--bg",
			"--https=" + strconv.Itoa(n.state.Services.Tailscale.Config.ServePort),
		}

		if n.state.Services.Tailscale.Config.ServeService != "" {
			args = append(args, "--service="+n.state.Services.Tailscale.Config.ServeService)
		}

		// tailscale serve is sensitive to argument order, so append the url last.
		args = append(args, "https+insecure://localhost:8443")

		_, err = subprocess.RunCommandContext(ctx, "tailscale", args...)
		if err != nil {
			return err
		}
	}

	return nil
}

// mapTailscalePeerToStatePeer maps the given Tailscale PeerStatus to a ServiceTailscaleStatePeer.
func mapTailscalePeerToStatePeer(peer *ipnstate.PeerStatus) api.ServiceTailscaleStatePeer {
	return api.ServiceTailscaleStatePeer{
		ID:           string(peer.ID),
		PublicKey:    peer.PublicKey.String(),
		HostName:     peer.HostName,
		DNSName:      peer.DNSName,
		OS:           peer.OS,
		TailscaleIPs: peer.TailscaleIPs,
		RxBytes:      peer.RxBytes,
		TxBytes:      peer.TxBytes,
		Online:       peer.Online,
		Expired:      peer.Expired,
	}
}

// parseTailscaleStatusJSON parses the given JSON data into a ServiceTailscaleState struct.
func parseTailscaleStatusJSON(jsonData []byte) (*api.ServiceTailscaleState, error) {
	// use the Tailscale ipnstate.Status struct to parse the JSON data
	var status ipnstate.Status

	// Unmarshal the JSON data into the ipnstate.Status struct.
	err := json.Unmarshal(jsonData, &status)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal tailscale status json data: %w", err)
	}

	// Get the tailnet name and DNS suffix if available.
	var tailnetName string

	var tailnetDNSSuffix string

	if status.CurrentTailnet != nil {
		tailnetName = status.CurrentTailnet.Name
		tailnetDNSSuffix = status.CurrentTailnet.MagicDNSSuffix
	}

	// Map Self PeerStatus to ServiceTailscaleStatePeer and do the same for each Peer in the Peer slice.
	var self api.ServiceTailscaleStatePeer
	if status.Self != nil {
		self = mapTailscalePeerToStatePeer(status.Self)
	}

	peers := make([]api.ServiceTailscaleStatePeer, 0, len(status.Peer))
	for _, peer := range status.Peer {
		if peer == nil {
			continue
		}

		peers = append(peers, mapTailscalePeerToStatePeer(peer))
	}

	// Sort the peers by HostName for consistent ordering.
	slices.SortFunc(peers, func(a, b api.ServiceTailscaleStatePeer) int {
		return strings.Compare(a.HostName, b.HostName)
	})

	// Construct the ServiceTailscaleState struct with the parsed data.
	tailscaleState := &api.ServiceTailscaleState{
		Version:          status.Version,
		BackendState:     api.ServiceTailscaleBackendStateEnum(status.BackendState),
		TailnetName:      tailnetName,
		TailnetDNSSuffix: tailnetDNSSuffix,
		Self:             self,
		Peers:            peers,
		HaveNodeKey:      status.HaveNodeKey,
		Health:           status.Health,
	}

	return tailscaleState, nil
}
