package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
)

func (s *Server) apiSystem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		endpoint, _ := url.JoinPath(getAPIRoot(r), "system")

		urls := []string{}

		for _, system := range []string{"logging", "network", "reset", "resources", "security", "storage", "update"} {
			systemURL, _ := url.JoinPath(endpoint, system)
			urls = append(urls, systemURL)
		}

		_ = response.SyncResponse(true, urls).Render(w)
	case http.MethodPut:
		type reqSystem struct {
			Action string `json:"action"`
		}

		var req reqSystem

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		switch req.Action {
		case "shutdown", "poweroff":
			close(s.state.TriggerShutdown)
		case "reboot":
			close(s.state.TriggerReboot)
		case "reset_encryption_bindings":
			err := secureboot.ForceUpdatePCRBindings(r.Context(), s.state.OS.Name, s.state.OS.RunningRelease, s.state.System.Security.Config.EncryptionRecoveryKeys[0])
			if err != nil {
				_ = response.InternalError(err).Render(w)

				return
			}
		default:
			_ = response.BadRequest(fmt.Errorf("invalid action %q", req.Action)).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)

		return
	}
}
