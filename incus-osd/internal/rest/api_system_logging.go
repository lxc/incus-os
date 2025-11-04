package rest

import (
	"encoding/json"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// swagger:operation GET /1.0/system/logging system system_get_logging
//
//	Get logging information
//
//	Returns the current system logging state and configuration information.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for the system logging
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
//	          description: State and configuration for the system logging
//	          example: {"config":{"syslog":{"address":"localhost","protocol":"TCP","log_format":""}},"state":{}}

// swagger:operation PUT /1.0/system/logging system system_put_logging
//
//	Update system logging configuration
//
//	Updates the system logging configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Logging configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The logging configuration
//	          example: {"syslog":{"address":"127.0.0.1","protocol":"TCP","log_format":""}}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemLogging(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return the current logging state.
		_ = response.SyncResponse(true, s.state.System.Logging).Render(w)
	case http.MethodPut:
		loggingData := &api.SystemLogging{}

		err := json.NewDecoder(r.Body).Decode(loggingData)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Apply new configuration
		err = systemd.SetSyslog(r.Context(), loggingData.Config.Syslog)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Persist the configuration.
		s.state.System.Logging.Config = loggingData.Config

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save()
}
