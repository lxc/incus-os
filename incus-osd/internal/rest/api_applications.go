package rest

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/lxc/incus-os/incus-osd/internal/applications"
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

func (s *Server) apiApplicationsFactoryReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	name := r.PathValue("name")

	// Check if the application is valid.
	_, ok := s.state.Applications[name]
	if !ok {
		_ = response.NotFound(nil).Render(w)

		return
	}

	// Load the application.
	app, err := applications.Load(r.Context(), s.state, name)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Do the factory reset.
	err = app.FactoryReset(r.Context())
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

func (s *Server) apiApplicationsBackup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	name := r.PathValue("name")

	// Check if the application is valid.
	_, ok := s.state.Applications[name]
	if !ok {
		_ = response.NotFound(nil).Render(w)

		return
	}

	// Load the application.
	app, err := applications.Load(r.Context(), s.state, name)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	complete := r.FormValue("complete")

	w.Header().Set("Content-Type", "application/x-tar")

	err = app.GetBackup(w, complete == "true")
	if err != nil {
		// This is unlikely to actually be a useful error, since we might
		// be in the middle of streaming a tar archive back when the error
		// is encountered.
		_ = response.BadRequest(err).Render(w)

		return
	}
}

func (s *Server) apiApplicationsRestore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	name := r.PathValue("name")

	// Check if the application is valid.
	_, ok := s.state.Applications[name]
	if !ok {
		_ = response.NotFound(nil).Render(w)

		return
	}

	// Load the application.
	app, err := applications.Load(r.Context(), s.state, name)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Restore the application's backup.
	err = app.RestoreBackup(r.Body)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Restart the application.
	err = app.Stop(r.Context(), "")
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	err = app.Start(r.Context(), "")
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}
