package rest

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
)

// swagger:operation GET /1.0/debug/storage debug debug_storage_get
//
//	Get storage debug endpoints
//
//	Returns a list of storage debug endpoints (URLs).
//
//	These endpoints have no guarantee of API stability, and should not be used in normal day-to-day operations.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API endpoints
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
//	          type: array
//	          description: List of storage debug endpoints
//	          items:
//	            type: string
//	          example: ["/1.0/debug/storage/:trace"]
func (*Server) apiDebugStorage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	debugURL, _ := url.JoinPath(getAPIRoot(r), "debug", "storage", ":trace")

	_ = response.SyncResponse(true, []string{debugURL}).Render(w)
}

// swagger:operation POST /1.0/debug/storage/:trace debug debug_storage_post_trace
//
//	Get a block device trace
//
//	Trace the I/O of a block device, streaming it as text until done or until the request is canceled.
//
//	The output matches that of blktrace. Only one trace can run at a time.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	  - application/octet-stream
//	parameters:
//	  - in: body
//	    name: arguments
//	    description: Block device trace arguments
//	    required: true
//	    schema:
//	      $ref: "#/definitions/DebugStorageTrace"
//	responses:
//	  "200":
//	    description: Text block device trace
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "409":
//	    $ref: "#/responses/Conflict"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugStorageTrace(w http.ResponseWriter, r *http.Request) {
	args := api.DebugStorageTrace{Duration: debugNetworkDefaultLimit}

	if !debugNetworkArgs(w, r, &args) {
		return
	}

	if args.Duration < 0 {
		_ = response.BadRequest(errors.New("invalid duration")).Render(w)

		return
	}

	ctx := r.Context()

	if args.Duration > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, time.Duration(args.Duration)*time.Second)
		defer cancel()
	}

	// The header only goes out with the first event, so errors can still be reported.
	w.Header().Set("Content-Type", "application/octet-stream")

	out := &debugFlushWriter{w: w, controller: http.NewResponseController(w)}

	err := storage.TraceBlockDevice(ctx, args.Device, args.Actions, out)
	if err == nil || out.written {
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch {
	case errors.Is(err, storage.ErrTraceInvalid):
		_ = response.BadRequest(err).Render(w)
	case errors.Is(err, storage.ErrTraceInProgress):
		_ = response.Conflict(err).Render(w)
	default:
		_ = response.InternalError(err).Render(w)
	}
}
