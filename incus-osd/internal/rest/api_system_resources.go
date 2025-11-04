package rest

import (
	"net/http"

	"github.com/lxc/incus/v6/shared/resources"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation GET /1.0/system/resources system system_get_resources
//
//	Get details about system resources
//
//	Returns a detailed low-level dump of the system's resources.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Low-level details about system resources
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
//	          type: json
//	          description: Low-level details about system resources
//	          example: {"cpu":{"architecture":"x86_64","sockets":[{"name":"AMD Ryzen Threadripper PRO 7965WX 24-Cores","vendor":"AuthenticAMD","socket":0,"cache":[{"level":1,"type":"Data","size":65536},{"level":1,"type":"Instruction","size":65536},{"level":2,"type":"Unified","size":524288},{"level":3,"type":"Unified","size":16777216}],"cores":[{"core":0,"die":0,"threads":[{"id":0,"numa_node":0,"thread":0,"online":true,"isolated":false}]}]}]}}
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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
