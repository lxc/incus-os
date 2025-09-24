package rest

import (
	"encoding/json"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/reset"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (*Server) apiSystemReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	resetData := &api.SystemReset{}

	if r.ContentLength > 0 {
		err := json.NewDecoder(r.Body).Decode(resetData)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}
	}

	err := reset.PerformOSFactoryReset(r.Context(), resetData)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Will never actually reach here, since the system will auto-reboot.
	_ = response.EmptySyncResponse.Render(w)
}
