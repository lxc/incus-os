package rest

import (
	"net/http"
	"net/url"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (*Server) apiSystem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	endpoint, _ := url.JoinPath(getAPIRoot(r), "system")

	urls := []string{}

	for _, system := range []string{"logging", "network", "resources", "security", "storage", "update"} {
		systemURL, _ := url.JoinPath(endpoint, system)
		urls = append(urls, systemURL)
	}

	_ = response.SyncResponse(true, urls).Render(w)
}

func (s *Server) apiSystemPoweroff(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	close(s.state.TriggerShutdown)

	_ = response.EmptySyncResponse.Render(w)
}

func (s *Server) apiSystemReboot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	close(s.state.TriggerReboot)

	_ = response.EmptySyncResponse.Render(w)
}
