package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

func (s *Server) apiSystemNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Refresh network state; needed to get current LLDP info.
		err := systemd.UpdateNetworkState(r.Context(), &s.state.System.Network)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Return the current network state.
		_ = response.SyncResponse(true, s.state.System.Network).Render(w)
	case http.MethodPut:
		// Replace the existing network configuration.
		newConfig := &api.SystemNetwork{}

		// Populate the network configuration from request's body.
		err := json.NewDecoder(r.Body).Decode(newConfig)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Don't allow a new configuration that doesn't define any interfaces, bonds, or vlans.
		if newConfig.Config == nil || seed.NetworkConfigHasEmptyDevices(*newConfig.Config) {
			_ = response.BadRequest(errors.New("network configuration has no devices defined")).Render(w)

			return
		}

		slog.InfoContext(r.Context(), "Applying new network configuration")

		err = systemd.ApplyNetworkConfiguration(r.Context(), s.state, newConfig.Config, 30*time.Second, false, providers.Refresh)
		if err != nil {
			slog.ErrorContext(r.Context(), "Failed to update network configuration: "+err.Error())
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
		_ = s.state.Save()
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}
}
