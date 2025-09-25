package rest

import (
	"errors"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/internal/backup"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (s *Server) apiSystemBackup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		archive, err := backup.GetOSBackup()
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		w.Header().Set("Content-Type", "application/x-tar")

		_, err = w.Write(archive)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}
	case http.MethodPatch, http.MethodPut:
		if r.ContentLength <= 0 {
			_ = response.BadRequest(errors.New("no tar archive provided")).Render(w)

			return
		}

		// PATCH will perform a partial restore; PUT will perform a complete restore.
		err := backup.ApplyOSBackup(r.Context(), s.state, r.Body, r.Method == http.MethodPut)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}
}
