package rest

import (
	"net/http"
	"strings"

	"github.com/lxc/incus-os/incus-osd/internal/backup"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (s *Server) apiSystemBackup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Make sure we have the current state written to disk prior to backup.
	err := s.state.Save()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	archive, err := backup.GetOSBackup()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	w.Header().Set("Content-Type", "application/gzip")

	_, err = w.Write(archive)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}
}

func (s *Server) apiSystemRestore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	skipString := r.FormValue("skip")
	skip := strings.Split(skipString, ",")

	err := backup.ApplyOSBackup(r.Context(), s.state, r.Body, skip)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}
