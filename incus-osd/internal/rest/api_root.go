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

func (s *Server) apiRoot10(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	resp := map[string]any{
		"environment": map[string]any{
			"hostname":   s.state.Hostname(),
			"os_name":    s.state.OS.Name,
			"os_version": s.state.OS.RunningRelease,
		},
	}

	_ = response.SyncResponse(true, resp).Render(w)
}
