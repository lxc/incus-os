package rest

import (
	"net/http"
	"strings"

	"github.com/lxc/incus-os/incus-osd/internal/backup"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation POST /1.0/system/:backup system systemd_post_backup
//
//	Generate a system backup
//
//	Generate and return a `gzip` compressed tar archive backup of the system state and configuration.
//
//	---
//	produces:
//	  - application/json
//	  - application/gzip
//	responses:
//	  "200":
//	    description: gzip'ed tar archive
//	    schema:
//	      type: file
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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

// swagger:operation POST /1.0/system/:restore system system_post_restore
//
//	Restore a system backup
//
//	Restore a `gzip` compressed tar backup of the system state and configuration. Upon completion the system will immediately reboot.
//
//	Remember to properly set the `Content-Type: application/gzip` HTTP header.
//
//	---
//	consumes:
//	  - application/gzip
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: gzip tar archive
//	    description: Application backup to restore
//	    required: true
//	    schema:
//	      type: file
//	  - in: query
//	    name: skip
//	    description: A comma-separated list of items to ignore when restoring the backup
//	    required: false
//	    type: array
//	    items:
//	      type: string
//	      enum:
//	        - encryption-recovery-keys
//	        - local-data-encryption-key
//	        - network-macs
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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
