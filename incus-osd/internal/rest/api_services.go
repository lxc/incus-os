package rest

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/services"
)

func (*Server) apiServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Get the list of services.
	urls := []string{}
	for _, service := range services.ValidNames {
		urls = append(urls, "/1.0/services/"+service)
	}

	_ = response.SyncResponse(true, urls).Render(w)
}

func (s *Server) apiServicesEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.PathValue("name")

	// Check if the service is valid.
	if !slices.Contains(services.ValidNames, name) {
		_ = response.NotFound(nil).Render(w)

		return
	}

	// Load the service.
	srv, err := services.Load(r.Context(), s.state, name)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	// Handle the request.
	switch r.Method {
	case http.MethodGet:
		resp, err := srv.Get(r.Context())
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.SyncResponse(true, resp).Render(w)

	case http.MethodPut:
		dest := srv.Struct()

		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(dest)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		err = srv.Update(r.Context(), dest)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		_ = response.NotImplemented(nil).Render(w)

		return
	}
}
