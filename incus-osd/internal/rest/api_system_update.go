package rest

import (
	"encoding/json"
	"net/http"

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
//	          $ref: "#/definitions/SystemUpdate"

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
//	      $ref: "#/definitions/SystemUpdate"
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

		// Ensure the new configuration is valid.
		err = newConfig.Config.Validate()
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
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
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: os_only
//	    description: If true, only check for OS updates
//	    required: false
//	    schema:
//	      type: object
//	      example: {"os_only": true}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
func (s *Server) apiSystemUpdateCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	type checkPost struct {
		OSOnly bool `json:"os_only"`
	}

	check := &checkPost{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(check)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Pick between a full update check and an OS-only one.
	trigger := s.state.TriggerUpdate
	if check.OSOnly {
		trigger = s.state.TriggerOSOnlyUpdate
	}

	// Non-blocking send so the request returns immediately even if a check is already pending.
	select {
	case trigger <- true:
	default:
	}

	_ = response.EmptySyncResponse.Render(w)
}
