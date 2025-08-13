package rest

import (
	"net/http"

	"github.com/lxc/incus/v6/shared/resources"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (*Server) apiSystemResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	resp, err := resources.GetResources()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.SyncResponse(true, resp).Render(w)
}
