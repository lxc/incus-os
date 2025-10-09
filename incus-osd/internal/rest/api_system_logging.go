package rest

import (
	"encoding/json"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

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
