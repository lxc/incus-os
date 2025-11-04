package rest

import (
	"encoding/json"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation GET /1.0/system/provider system system_get_provider
//
//	Get provider information
//
//	Returns the current system provider state and configuration information.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for the system provider
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
//	          description: State and configuration for the system provider
//	          example: {"config":{"name":"images","config":null},"state":{"registered":false}}

// swagger:operation PUT /1.0/system/provider system system_put_provider
//
//	Update system provider configuration
//
//	Updates the system provider configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Provider configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The provider configuration
//	          example: {"name":"images","config":null}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return the current system provider state.
		_ = response.SyncResponse(true, s.state.System.Provider).Render(w)
	case http.MethodPut:
		// Apply a new system provider configuration.
		newConfig := &api.SystemProvider{}
		oldConfig := s.state.System.Provider.Config

		// Update the system provider configuration from request's body.
		err := json.NewDecoder(r.Body).Decode(newConfig)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Load the current provider and deregister it.
		p, err := providers.Load(r.Context(), s.state)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		err = p.Deregister(r.Context())
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Apply the updated configuration.
		s.state.System.Provider.Config = newConfig.Config

		// Load the new provider and register it.
		p, err = providers.Load(r.Context(), s.state)
		if err != nil {
			s.state.System.Provider.Config = oldConfig
			_ = s.state.Save()
			_ = response.InternalError(err).Render(w)

			return
		}

		err = p.Register(r.Context(), false)
		if err != nil {
			s.state.System.Provider.Config = oldConfig
			_ = s.state.Save()
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
