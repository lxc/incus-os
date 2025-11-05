package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation GET /1.0/system/update system system_get_update
//
//	Get update information
//
//	Returns the current system update state and configuration information.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for the system update
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
//	          description: State and configuration for the system update
//	          example: {"config":{"auto_reboot":false,"channel":"stable","check_frequency":"6h"},"state":{"last_check":"2025-11-04T16:21:34.929524792Z","status":"Update check completed","needs_reboot":false}}

// swagger:operation PUT /1.0/system/update system system_put_update
//
//	Update system update configuration
//
//	Updates the system update configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Update configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The update configuration
//	          example: {"auto_reboot":false,"channel":"testing","check_frequency":"1d"}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
func (s *Server) apiSystemUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return the current system update state.
		_ = response.SyncResponse(true, s.state.System.Update).Render(w)
	case http.MethodPut:
		// Apply a new system update configuration.
		newConfig := &api.SystemUpdate{}

		// Update the system update configuration from request's body.
		err := json.NewDecoder(r.Body).Decode(newConfig)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Basic validation.
		for _, mw := range newConfig.Config.MaintenanceWindows {
			// To simplify logic, we don't allow a week-long migration window
			// to start and end on the same day.
			if mw.StartDayOfWeek != api.NONE && mw.StartDayOfWeek == mw.EndDayOfWeek {
				if mw.EndHour*60+mw.EndMinute < mw.StartHour*60+mw.StartMinute {
					_ = response.BadRequest(errors.New("invalid migration window: end time is before start time")).Render(w)

					return
				}
			}

			// If either StartDayOfWeek or EndDayOfWeek is specified, the other must be too.
			if (mw.StartDayOfWeek == api.NONE && mw.EndDayOfWeek != api.NONE) || (mw.StartDayOfWeek != api.NONE && mw.EndDayOfWeek == api.NONE) {
				_ = response.BadRequest(errors.New("invalid migration window: both StartDayOfWeek and EndDayOfWeek must be provided")).Render(w)

				return
			}
		}

		// Check the update frequency is valid.
		if newConfig.Config.CheckFrequency != "never" {
			_, err = time.ParseDuration(newConfig.Config.CheckFrequency)
			if err != nil {
				_ = response.BadRequest(errors.New("invalid update check frequency")).Render(w)

				return
			}
		}

		// Apply the updated configuration.
		s.state.System.Update.Config = newConfig.Config

		_ = response.EmptySyncResponse.Render(w)

		_ = s.state.Save()
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}
}

// swagger:operation POST /1.0/system/update/:check system system_post_update_check
//
//	Trigger update check
//
//	Triggers an immediate system update check.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
func (s *Server) apiSystemUpdateCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Trigger a manual update check.
	s.state.TriggerUpdate <- true

	_ = response.EmptySyncResponse.Render(w)
}
