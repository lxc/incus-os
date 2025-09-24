package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

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
