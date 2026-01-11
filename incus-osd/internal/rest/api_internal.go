package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/auth"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (*Server) apiInternalTUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get("X-IncusOS-Proxy") != "" {
		_ = response.Forbidden(nil).Render(w)

		return
	}

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

func (s *Server) apiInternalRegistration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get("X-IncusOS-Proxy") != "" {
		_ = response.Forbidden(nil).Render(w)

		return
	}

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	type apiPost struct {
		Token string `json:"token"`
	}

	req := &apiPost{}

	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if req.Token == "" {
		_ = response.BadRequest(errors.New("missing registration token")).Render(w)

		return
	}

	machineID, err := s.state.MachineID()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	reg, err := auth.GenerateRegistration(r.Context(), machineID, req.Token)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.SyncResponse(true, reg).Render(w)
}

func (s *Server) apiInternalToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get("X-IncusOS-Proxy") != "" {
		_ = response.Forbidden(nil).Render(w)

		return
	}

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	machineID, err := s.state.MachineID()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	token, err := auth.GenerateToken(r.Context(), machineID)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.SyncResponse(true, token).Render(w)
}
