package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/power"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation GET /1.0/system/power system system_get_power
//
//	Get power management configuration
//
//	Returns the current power management configuration and state.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for power management
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
//	          $ref: "#/definitions/SystemPower"

// swagger:operation PUT /1.0/system/power system system_put_power
//
//	Update power management configuration
//
//	Updates the power management configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Power management configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/SystemPower"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemPower(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Get current state for power.
		powerState, err := power.GetPowerState(r.Context())
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Update power state.
		s.state.System.Power.State = powerState

		// Return the current power state.
		_ = response.SyncResponse(true, s.state.System.Power).Render(w)
	case http.MethodPut:
		powerData := api.SystemPower{}

		err := json.NewDecoder(r.Body).Decode(&powerData)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Apply new power management configuration.
		err = power.ApplyPowerConfiguration(r.Context(), powerData.Config)
		if err != nil {
			if errors.Is(err, power.ErrInvalidConfig) {
				_ = response.BadRequest(err).Render(w)

				return
			}

			_ = response.InternalError(err).Render(w)

			return
		}

		// Update power management config.
		s.state.System.Power.Config = powerData.Config
		_ = response.EmptySyncResponse.Render(w)

	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save()
}
