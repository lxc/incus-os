package rest

import (
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

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

func (s *Server) apiApplicationsRestart(w http.ResponseWriter, r *http.Request) {
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

	// Trigger the restart.
	err = app.Restart(r.Context(), s.state.Applications[name].State.Version)
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

	// Once we begin streaming the tar archive back to the user,
	// we can no longer return a nice error message if something
	// goes wrong. So, first generate the archive and dump everything
	// to /dev/null. If any error is reported, we can return it to the
	// user. We can't buffer in-memory or on-disk since we don't know
	// how large the archive might be and we don't want to DOS ourselves.
	err = app.GetBackup(io.Discard, complete == "true")
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	w.Header().Set("Content-Type", "application/gzip")

	// From this point onwards we cannot return any nice errors
	// to the user, since we will have already begun streaming
	// the tar archive to them.

	err = app.GetBackup(w, complete == "true")
	if err != nil {
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
	appInfo, ok := s.state.Applications[name]
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
	err = app.RestoreBackup(r.Context(), r.Body)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Record when the application was restored.
	now := time.Now()
	appInfo.State.LastRestored = &now
	s.state.Applications[name] = appInfo

	err = s.state.Save()
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}
