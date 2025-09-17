package rest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"

	"github.com/lxc/incus-os/incus-osd/api"
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

	name := r.PathValue("name")

	// Check if the application is valid.
	app, ok := s.state.Applications[name]
	if !ok {
		_ = response.NotFound(nil).Render(w)

		return
	}

	switch r.Method {
	case http.MethodGet:
		// Handle the request.
		_ = response.SyncResponse(true, app).Render(w)
	case http.MethodDelete:
		// Uninstall the application.
		if r.ContentLength <= 0 {
			_ = response.BadRequest(errors.New("no uninstall parameters provided")).Render(w)

			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		deleteStruct := api.ApplicationDelete{}

		err = json.Unmarshal(b, &deleteStruct)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		actualApp, err := applications.Load(r.Context(), name)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		err = actualApp.Uninstall(r.Context(), deleteStruct.RemoveUserData)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Remove the application from state and then write to disk.
		delete(s.state.Applications, name)

		err = s.state.Save(r.Context())
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}
}
