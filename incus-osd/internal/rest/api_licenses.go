package rest

import (
	"net/http"
	"net/url"

	"github.com/lxc/incus-os/incus-osd/internal/licenses"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation GET /1.0/licenses licenses licenses_get
//
//	Get available license information
//
//	Returns a list of everything on the system for which license information is available (URLs).
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
//	          description: List of license entries
//	          items:
//	            type: string
//	          example: ["/1.0/licenses/local:incus-osd","/1.0/licenses/debian:systemd","/1.0/licenses/license:GPL-2"]
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiLicenses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Get the license information.
	info, err := licenses.Load(licenses.Dir)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	endpoint, _ := url.JoinPath(getAPIRoot(r), "licenses")

	urls := []string{}

	for _, name := range info.Names() {
		licenseURL, _ := url.JoinPath(endpoint, name)
		urls = append(urls, licenseURL)
	}

	_ = response.SyncResponse(true, urls).Render(w)
}

// swagger:operation GET /1.0/licenses/{name} licenses licenses_get_license
//
//	Get the license information for a component
//
//	Returns the copyright and license text of a Debian package (`debian:NAME`),
//	of other software included in the system (`local:NAME`) or the plain text of a
//	common license (`license:NAME`).
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: License entry name
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    description: License information
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
//	          $ref: "#/definitions/License"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiLicensesEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Get the license information.
	info, err := licenses.Load(licenses.Dir)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	license := info.Get(r.PathValue("name"))
	if license == nil {
		_ = response.NotFound(nil).Render(w)

		return
	}

	_ = response.SyncResponse(true, license).Render(w)
}
