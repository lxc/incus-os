package rest

import (
	"errors"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/services"
)

// swagger:operation GET /1.0/debug/lvm debug debug_get_lvm
//
//	Get LVM debug information
//
//	Return the full physical volume, volume group and logical volume reports.
//
//	The reports are read from the on-disk LVM metadata only, without taking any lock, so device status fields aren't populated.
//	All sizes are in bytes.
//
//	This endpoint is only available when the LVM service is enabled.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: LVM debug information
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/DebugLVM"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiDebugLVM(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	if !s.state.Services.LVM.Config.Enabled {
		_ = response.BadRequest(errors.New("LVM service isn't enabled")).Render(w)

		return
	}

	srv, err := services.Load(r.Context(), s.state, "lvm")
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	info, err := srv.Debug(r.Context())
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.SyncResponse(true, info).Render(w)
}
