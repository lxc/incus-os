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

// swagger:operation GET /1.0/system/network system system_get_network
//
//	Get network information
//
//	Returns the current system network state and configuration information.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for the system network
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: json
//	          description: State and configuration for the system network
//	          example: {"config":{"interfaces":[{"name":"enp5s0","addresses":["dhcp4","slaac"],"required_for_online":"no","hwaddr":"10:66:6a:1a:20:0f","lldp":false}],"time":{"timezone":"UTC"}},"state":{"interfaces":{"enp5s0":{"type":"interface","addresses":["10.234.136.149","fd42:3cfb:8972:3990:1266:6aff:fe1a:200f"],"hwaddr":"10:66:6a:1a:20:0f","routes":[{"to":"default","via":"10.234.136.1"}],"mtu":1500,"speed":"-1","state":"routable","stats":{"rx_bytes":82290,"tx_bytes":43500,"rx_errors":0,"tx_errors":0},"roles":["management","cluster"]}}}}
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation PUT /1.0/system/network system system_put_network
//
//	Update system network configuration
//
//	Updates the system network configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Network configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The network configuration
//	          example: {"interfaces":[{"name":"enp5s0","addresses":["dhcp4"],"required_for_online":"yes","hwaddr":"10:66:6a:1a:20:0f","lldp":true}],"time":{"timezone":"America/New_York"}}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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

		// If no timezone has been set, default to UTC.
		if s.state.System.Network.Config.Time == nil {
			s.state.System.Network.Config.Time = &api.SystemNetworkTime{}
		}

		if s.state.System.Network.Config.Time.Timezone == "" {
			s.state.System.Network.Config.Time.Timezone = "UTC"
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
