package rest

import (
	"encoding/json"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/kernel"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation GET /1.0/system/kernel system system_get_kernel
//
//	Get kernel-level configuration information
//
//	Returns the current kernel-level configuration information.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for the system kernel
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
//	          description: State and configuration for the system kernel
//	          example: {"config":{"blacklist_modules":["bad-module"],"network":{"buffer_size":33554432,"queuing_discipline":"fq","tcp_congestion_algorithm":"bbr"},"pci":{"passthrough":[{"vendor_id":"1af4","product_id":"1050","pci_address":"0000:04:00.0"}]}}}

// swagger:operation PUT /1.0/system/kernel system system_put_kernel
//
//	Update system kernel-level configuration
//
//	Updates the system kernel-level configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Kernel configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The kernel configuration
//	          example: {"blacklist_modules":["bad-module"],"network":{"buffer_size":33554432,"queuing_discipline":"fq","tcp_congestion_algorithm":"bbr"},"pci":{"passthrough":[{"vendor_id":"1af4","product_id":"1050","pci_address":"0000:04:00.0"}]}}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemKernel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return the current kernel state.
		_ = response.SyncResponse(true, s.state.System.Kernel).Render(w)
	case http.MethodPut:
		kernelData := &api.SystemKernel{}

		err := json.NewDecoder(r.Body).Decode(kernelData)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Apply new configuration
		err = kernel.ApplyKernelConfiguration(r.Context(), kernelData.Config)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Persist the configuration.
		s.state.System.Kernel.Config = kernelData.Config

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save()
}
