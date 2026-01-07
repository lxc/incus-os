package rest

import (
	"net/http"
	"net/url"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// If the request contains a X-IncusOS-Proxy header, prepend that to
// the API root that's returned to the user.
func getAPIRoot(r *http.Request) string {
	if r == nil {
		return "/1.0"
	}

	prefix := r.Header.Get("X-IncusOS-Proxy")
	if prefix == "" {
		prefix = "/"
	}

	ret, _ := url.JoinPath(prefix, "1.0")

	return ret
}

// swagger:operation GET / server api_get
//
//	Get the supported API endpoints
//
//	Returns a list of supported API versions (URLs).
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
//	          description: List of endpoints
//	          items:
//	            type: string
//	          example: ["/1.0"]
func (*Server) apiRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	if r.URL.Path != "/" {
		_ = response.NotFound(nil).Render(w)

		return
	}

	_ = response.SyncResponse(true, []string{getAPIRoot(r)}).Render(w)
}

// swagger:operation GET /1.0 server server_get
//
//	Get basic information about the server environment
//
//	Returns a map with basic information about the server's environment.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Basic server environment
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
//	          description: Basic server information
//	          example: {"environment":{"hostname":"af94e64e-1993-41b6-8f10-a8eebb828fce","os_name":"IncusOS","os_version":"202511041601","os_version_next":202511152230}}
func (s *Server) apiRoot10(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	resp := map[string]any{
		"environment": map[string]any{
			"hostname":        s.state.Hostname(),
			"os_name":         s.state.OS.Name,
			"os_version":      s.state.OS.RunningRelease,
			"os_version_next": s.state.OS.NextRelease,
		},
	}

	_ = response.SyncResponse(true, resp).Render(w)
}
