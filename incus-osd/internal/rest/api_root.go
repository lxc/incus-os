package rest

import (
	"net/http"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

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

	_ = response.SyncResponse(true, []string{"/1.0"}).Render(w)
}

func (s *Server) apiRoot10(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	resp := map[string]any{
		"environment": map[string]any{
			"os_version":   s.state.RunningRelease,
			"applications": s.state.Applications,
		},
	}

	_ = response.SyncResponse(true, resp).Render(w)
}
