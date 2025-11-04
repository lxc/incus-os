package rest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/services"
)

// swagger:operation GET /1.0/services services services_get
//
//	Get available services
//
//	Returns a list of currently available services (URLs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API endpoints
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
//	          type: array
//	          description: List of services
//	          items:
//	            type: string
//	          example: ["/1.0/services/ceph","/1.0/services/iscsi","/1.0/services/lvm","/1.0/services/multipath","/1.0/services/nvme","/1.0/services/ovn","/1.0/services/tailscale","/1.0/services/usbip"]
func (s *Server) apiServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Get the list of services.
	names := slices.Clone(services.Supported(s.state))
	slices.Sort(names)

	endpoint, _ := url.JoinPath(getAPIRoot(r), "services")

	urls := []string{}

	for _, service := range names {
		serviceURL, _ := url.JoinPath(endpoint, service)
		urls = append(urls, serviceURL)
	}

	_ = response.SyncResponse(true, urls).Render(w)
}

// swagger:operation GET /1.0/services/{name} services services_get_service
//
//	Get service-specific information
//
//	Returns service-specific state and configuration information.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Service name
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    description: State and configuration for the service
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
//	          description: State and configuration for the service
//	          example: {"state":{},"config":{"enabled":false,"system_id":0}}
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation PUT /1.0/services/{name} services services_put_service
//
//	Update service configuration
//
//	Updates a service's configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Service name
//	    required: true
//	    type: string
//	  - in: body
//	    name: configuration
//	    description: Service configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The service configuration
//	          example: {"enabled":true,"system_id":123}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiServicesEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := r.PathValue("name")

	// Check if the service is valid.
	if !slices.Contains(services.Supported(s.state), name) {
		_ = response.NotFound(nil).Render(w)

		return
	}

	// Load the service.
	srv, err := services.Load(r.Context(), s.state, name)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	// Handle the request.
	switch r.Method {
	case http.MethodGet:
		resp, err := srv.Get(r.Context())
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.SyncResponse(true, resp).Render(w)

	case http.MethodPut:
		dest := srv.Struct()

		decoder := json.NewDecoder(r.Body)

		err = decoder.Decode(dest)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		err = srv.Update(r.Context(), dest)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		_ = response.NotImplemented(nil).Render(w)

		return
	}
}

// swagger:operation POST /1.0/services/{name}/:reset services services_post_reset
//
//	Forcefully reset service
//
//	Forcefully resets the service.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Service name
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiServicesEndpointReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := r.PathValue("name")

	// Check if the service is valid.
	if !slices.Contains(services.Supported(s.state), name) {
		_ = response.NotFound(nil).Render(w)

		return
	}

	// Load the service.
	srv, err := services.Load(r.Context(), s.state, name)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	// Handle the request.
	switch r.Method {
	case http.MethodPost:
		err = srv.Reset(r.Context())
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		_ = response.NotImplemented(nil).Render(w)

		return
	}
}
