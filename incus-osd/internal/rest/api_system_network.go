package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

func (s *Server) apiSystemNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return the current network configuration.
		_ = response.SyncResponse(true, s.state.System.Network).Render(w)
	case http.MethodPatch, http.MethodPut:
		// Apply an update or completely replace the network configuration.
		newConfig := new(api.SystemNetwork)

		// If updating, grab the current configuration.
		if r.Method == http.MethodPatch {
			// We make a copy of the current network configuration so we don't corrupt
			// the existing good state with a bad update from the user.
			cpy, err := json.Marshal(s.state.System.Network)
			if err != nil {
				_ = response.BadRequest(err).Render(w)

				return
			}

			err = json.Unmarshal(cpy, newConfig)
			if err != nil {
				_ = response.BadRequest(err).Render(w)

				return
			}
		}

		// Update the network configuration from request's body.
		err := json.NewDecoder(r.Body).Decode(newConfig)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Don't allow a new configuration that doesn't define any interfaces, bonds, or vlans.
		if seed.NetworkConfigHasEmptyDevices(*newConfig) {
			_ = response.BadRequest(errors.New("network configuration has no devices defined")).Render(w)

			return
		}

		// Apply the updated configuration.
		s.state.System.Network = newConfig
		err = systemd.ApplyNetworkConfiguration(r.Context(), s.state.System.Network, 10*time.Second)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)

		_ = s.state.Save(r.Context())
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}
}
