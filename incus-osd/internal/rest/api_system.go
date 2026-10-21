package rest

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// swagger:operation GET /1.0/system system system_get
//
//	Get list of system endpoints
//
//	Returns a list of system endpoints (URLs).
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
//	          description: List of system endpoints
//	          items:
//	            type: string
//	          example: ["/1.0/system/logging","/1.0/system/network","/1.0/system/provider","/1.0/system/resources","/1.0/system/security","/1.0/system/storage","/1.0/system/update"]
func (*Server) apiSystem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	endpoint, _ := url.JoinPath(getAPIRoot(r), "system")

	urls := []string{}

	for _, system := range []string{"kernel", "logging", "network", "provider", "resources", "security", "storage", "update"} {
		systemURL, _ := url.JoinPath(endpoint, system)
		urls = append(urls, systemURL)
	}

	_ = response.SyncResponse(true, urls).Render(w)
}

// swagger:operation POST /1.0/system/:poweroff system system_post_poweroff
//
//	Power off the system
//
//	Powers off the system.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: version
//	    description: Optionally, specify next system boot version
//	    required: false
//	    schema:
//	      $ref: "#/definitions/SystemNextBootVersion"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemPoweroff(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	version := &api.SystemNextBootVersion{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(version)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	switch version.NextBootVersion {
	case "current":
		err = util.SetNextBootID(r.Context(), util.BootIDCurrent)
	case "oldest":
		err = util.SetNextBootID(r.Context(), util.BootIDOldest)
	default:
		err = util.SetNextBootID(r.Context(), util.BootIDNewest)
	}

	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	s.state.TriggerShutdown <- true

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/system/:reboot system system_post_reboot
//
//	Reboot the system
//
//	Reboots the system.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: version
//	    description: Optionally, specify next system boot version
//	    required: false
//	    schema:
//	      $ref: "#/definitions/SystemNextBootVersion"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemReboot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	version := &api.SystemNextBootVersion{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(version)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	switch version.NextBootVersion {
	case "current":
		err = util.SetNextBootID(r.Context(), util.BootIDCurrent)
	case "oldest":
		err = util.SetNextBootID(r.Context(), util.BootIDOldest)
	default:
		err = util.SetNextBootID(r.Context(), util.BootIDNewest)
	}

	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	s.state.TriggerReboot <- true

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/system/:suspend system system_post_suspend
//
//	Suspend the system
//
//	Suspends the system.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
func (s *Server) apiSystemSuspend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	s.state.TriggerSuspend <- true

	_ = response.EmptySyncResponse.Render(w)
}
