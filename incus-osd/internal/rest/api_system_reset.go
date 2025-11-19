package rest

import (
	"encoding/json"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/reset"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
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
