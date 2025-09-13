package rest

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (s *Server) apiApplications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Get the list of services.
	names := make([]string, 0, len(s.state.Applications))

	for name := range s.state.Applications {
		names = append(names, name)
	}

	slices.Sort(names)

	endpoint, _ := url.JoinPath(getAPIRoot(r), "applications")

	urls := []string{}

	for _, application := range names {
		appURL, _ := url.JoinPath(endpoint, application)
		urls = append(urls, appURL)
	}

	_ = response.SyncResponse(true, urls).Render(w)
}

func (s *Server) apiApplicationsEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	name := r.PathValue("name")

	// Check if the application is valid.
	app, ok := s.state.Applications[name]
	if !ok {
		_ = response.NotFound(nil).Render(w)

		return
	}

	// Handle the request.
	_ = response.SyncResponse(true, app).Render(w)
}
