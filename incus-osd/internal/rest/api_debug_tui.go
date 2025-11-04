package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation POST /1.0/debug/tui/:write-message debug debug_post_tui_write_log
//
//	Log a message
//
//	Send a message that should be logged by the system.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: message
//	    description: Message to be logged
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        level:
//	          type: string
//	          description: The log level
//	          example: INFO
//	        message:
//	          type: string
//	          description: The log message
//	          example: Hello, world
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
func (*Server) apiDebugTUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	logMessage := &api.DebugTUI{}

	err := json.NewDecoder(r.Body).Decode(logMessage)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if logMessage.Message == "" {
		_ = response.BadRequest(errors.New("no log message provided")).Render(w)

		return
	}

	switch {
	case logMessage.Level < slog.LevelInfo:
		slog.DebugContext(r.Context(), logMessage.Message)
	case logMessage.Level < slog.LevelWarn:
		slog.InfoContext(r.Context(), logMessage.Message)
	case logMessage.Level < slog.LevelError:
		slog.WarnContext(r.Context(), logMessage.Message)
	default:
		slog.ErrorContext(r.Context(), logMessage.Message)
	}

	_ = response.EmptySyncResponse.Render(w)
}
