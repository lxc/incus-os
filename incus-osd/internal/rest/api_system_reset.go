package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/reset"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/update"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// swagger:operation POST /1.0/system/:factory-reset system system_post_reset
//
//	Perform a factory reset of the system
//
//	Factory reset the entire system and immediately reboot. This is a DESTRUCTIVE action and will wipe all installed applications, configuration, and the "local" ZFS datapool.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Reset data
//	    required: false
//	    schema:
//	      type: object
//	      example: {"allow_tpm_reset_failure":false,"wipe_existing_seeds":true,"seeds":{"incus":{"apply_defaults":true}}}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiSystemFactoryReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	resetData := &api.SystemReset{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(resetData)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	err = reset.PerformOSFactoryReset(r.Context(), resetData)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	// Will never actually reach here, since the system will auto-reboot.
	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/system/:reinstall-inactive-os system system_post_reinstall_inactive_os
//
//	Reinstall the inactive version of IncusOS.
//
//	Triggers a reinstall of the inactive A/B /usr-verity partitions used by IncusOS. It is not possible to
//	reinstall the currently running version of IncusOS.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemReinstallInactiveOS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Get the list of currently installed UKI versions.
	ukiVersions, err := util.GetUKIVersions()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	if ukiVersions.OtherVersion == "" {
		_ = response.BadRequest(errors.New("no other version of " + s.state.OS.Name + " available to reinstall")).Render(w)

		return
	}

	// Perform a forceful reinstall of the other version of IncusOS that is installed. This may be
	// the prior booted version, or a pending update that hasn't booted yet.
	err = update.CheckOS(r.Context(), s.state, ukiVersions.OtherVersion, false, true)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}
