package rest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/update"
)

// swagger:operation GET /1.0/applications applications applications_get
//
//	Get currently installed applications
//
//	Returns a list of currently installed applications (URLs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API endpoints
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: List of applications
//	          items:
//	            type: string
//	          example: ["/1.0/applications/incus"]

// swagger:operation POST /1.0/applications applications applications_post
//
//	Add an application
//
//	Installs a new application on the system.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: application
//	    description: Application to be installed
//	    required: true
//	    schema:
//	      type: object
//	      example: {"name": "gpu-support"}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "409":
//	    $ref: "#/responses/Conflict"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiApplications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Get the list of applications.
		names := make([]string, 0, len(s.state.Applications))

		for name := range s.state.Applications {
			names = append(names, name)
		}

		slices.Sort(names)

		endpoint, _ := url.JoinPath(getAPIRoot(r), "applications")

		urls := make([]string, 0, len(names))

		for _, application := range names {
			appURL, _ := url.JoinPath(endpoint, application)
			urls = append(urls, appURL)
		}

		_ = response.SyncResponse(true, urls).Render(w)

	case http.MethodPost:
		type applicationPost struct {
			Name string `json:"name"`
		}

		app := &applicationPost{}

		counter := &countWrapper{ReadCloser: r.Body}

		err := json.NewDecoder(counter).Decode(app)
		if err != nil && counter.n > 0 {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Input validation.
		if app.Name == "" {
			_ = response.BadRequest(errors.New("missing application name")).Render(w)

			return
		}

		// Check if the application is already installed.
		_, exists := s.state.Applications[app.Name]
		if exists {
			_ = response.Conflict(nil).Render(w)

			return
		}

		// Don't allow more than one primary application to be installed.
		actualApp, err := applications.Load(r.Context(), s.state, app.Name)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		if actualApp.IsPrimary() {
			_ = response.BadRequest(errors.New("a primary application is already installed")).Render(w)

			return
		}

		// Install the application.
		err = update.InstallUpdateApp(r.Context(), s.state, app.Name, false)
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

// swagger:operation GET /1.0/applications/{name} applications applications_get_application
//
//	Get application-specific information
//
//	Returns application-specific state and configuration information.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Application name
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    description: State and configuration for the application
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: json
//	          description: State and configuration for the application
//	          example: {"state":{"initialized":true,"version":"202511041800","available_versions":["202511041601","202511041800"]},"config":{}}
//	  "404":
//	    $ref: "#/responses/NotFound"
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

// swagger:operation POST /1.0/applications/{name}/:debug applications applications_post_debug
//
//	Perform debug actions or retrieve debug data for an application
//
//	Triggers a debug action or debug data capture.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Application name
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiApplicationsDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")

		_ = response.NotImplemented(nil).Render(w)

		return
	}

	name := r.PathValue("name")

	// Check if the application is valid.
	_, ok := s.state.Applications[name]
	if !ok {
		w.Header().Set("Content-Type", "application/json")

		_ = response.NotFound(nil).Render(w)

		return
	}

	// Load the application.
	app, err := applications.Load(r.Context(), s.state, name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")

		_ = response.InternalError(err).Render(w)

		return
	}

	// Trigger the debug call.
	dest := app.DebugStruct()
	decoder := json.NewDecoder(r.Body)

	err = decoder.Decode(dest)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")

		_ = response.InternalError(err).Render(w)

		return
	}

	_ = app.Debug(r.Context(), dest).Render(w)
}

// swagger:operation POST /1.0/applications/{name}/:factory-reset applications applications_post_reset
//
//	Perform a factory reset of the application
//
//	Factory reset the application. This is a DESTRUCTIVE action and will wipe any local application configuration.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Application name
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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
		_ = response.InternalError(err).Render(w)

		return
	}

	// Do the factory reset.
	err = app.FactoryReset(r.Context())
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/applications/{name}/:restart applications applications_post_restart
//
//	Restart an application
//
//	Triggers a restart of the application.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Application name
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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
		_ = response.InternalError(err).Render(w)

		return
	}

	// Trigger the restart.
	err = app.Restart(r.Context())
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/applications/{name}/:backup applications applications_post_backup
//
//	Generate an application backup
//
//	Generate and return a `gzip` compressed tar archive backup for the application.
//
//	A full backup may be quite large depending on what artifacts or updates are locally cached by the application.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	  - application/gzip
//	parameters:
//	  - in: path
//	    name: name
//	    description: Application name
//	    required: true
//	    type: string
//	  - in: body
//	    name: configuration
//	    description: Backup configuration
//	    required: false
//	    schema:
//	      type: object
//	      example: {"complete":true}
//	responses:
//	  "200":
//	    description: gzip'ed tar archive
//	    schema:
//	      type: file
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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
		_ = response.InternalError(err).Render(w)

		return
	}

	type backupStruct struct {
		Complete bool `json:"complete"`
	}

	config := &backupStruct{}

	counter := &countWrapper{ReadCloser: r.Body}

	err = json.NewDecoder(counter).Decode(config)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Once we begin streaming the tar archive back to the user,
	// we can no longer return a nice error message if something
	// goes wrong. So, first generate the archive and dump everything
	// to /dev/null. If any error is reported, we can return it to the
	// user. We can't buffer in-memory or on-disk since we don't know
	// how large the archive might be and we don't want to DOS ourselves.
	err = app.GetBackup(io.Discard, config.Complete)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	w.Header().Set("Content-Type", "application/gzip")

	// From this point onwards we cannot return any nice errors
	// to the user, since we will have already begun streaming
	// the tar archive to them.

	err = app.GetBackup(w, config.Complete)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}
}

// swagger:operation POST /1.0/applications/{name}/:restore applications applications_post_restore
//
//	Restore an application backup
//
//	Restore a `gzip` compressed tar archive backup for the application. After a successful restore, the application will be restarted.
//
//	Remember to properly set the `Content-Type: application/gzip` HTTP header.
//
//	---
//	consumes:
//	  - application/gzip
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Application name
//	    required: true
//	    type: string
//	  - in: body
//	    name: gzip tar archive
//	    description: Application backup to restore
//	    required: true
//	    schema:
//	      type: file
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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
		_ = response.InternalError(err).Render(w)

		return
	}

	// Restore the application's backup.
	err = app.RestoreBackup(r.Context(), r.Body)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	// Record when the application was restored.
	now := time.Now()
	appInfo.State.LastRestored = &now
	s.state.Applications[name] = appInfo

	err = s.state.Save()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/applications/{name}/:remove applications applications_post_remove
//
//	Remove an application
//
//	Uninstall a non-primary application that is currently installed.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Application to be removed
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiApplicationsRemove(w http.ResponseWriter, r *http.Request) {
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

	// Remove the application.
	err := applications.UninstallApplication(r.Context(), s.state, name)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/applications/{name}/:check-update applications applications_post_check_update
//
//	Check for application update
//
//	Trigger a manual check for an update of the given application, and apply it if found.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: name
//	    description: Application name
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiApplicationsCheckUpdate(w http.ResponseWriter, r *http.Request) {
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

	// Check for and apply application update.
	err := update.InstallUpdateApp(r.Context(), s.state, name, true)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/applications/{name}/:switch-version applications applications_post_switch_version
//
//	Configure the version of the application
//
//	Switch the version of the application to a version available locally on disk. If no version is
//	specified, attempt to rollback to the prior available version of the application.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: application
//	    description: Optional exact version to switch to
//	    required: false
//	    schema:
//	      type: object
//	      example: {"version": "202603201831"}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiApplicationsSwitchVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
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

	// Get the specific application version to switch to, if specified.
	type versionInfo struct {
		Version string `json:"version"`
	}

	vi := &versionInfo{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(vi)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Switch the application version.
	err = app.SwitchVersion(vi.Version)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	// Reload sysext images to reload application.
	err = systemd.RefreshExtensions(r.Context(), s.state.Applications, &s.state.OS)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}
