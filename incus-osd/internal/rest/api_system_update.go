package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

func (s *Server) apiSystemUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return the current system update state.
		_ = response.SyncResponse(true, s.state.System.Update).Render(w)
	case http.MethodPut:
		// Apply a new system update configuration.
		newConfig := &api.SystemUpdate{}

		// Update the system update configuration from request's body.
		err := json.NewDecoder(r.Body).Decode(newConfig)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Basic validation.
		for _, mw := range newConfig.Config.MaintenanceWindows {
			// To simplify logic, we don't allow a week-long migration window
			// to start and end on the same day.
			if mw.StartDayOfWeek != api.NONE && mw.StartDayOfWeek == mw.EndDayOfWeek {
				if mw.EndHour*60+mw.EndMinute < mw.StartHour*60+mw.StartMinute {
					_ = response.BadRequest(errors.New("invalid migration window: end time is before start time")).Render(w)

					return
				}
			}

			// If either StartDayOfWeek or EndDayOfWeek is specified, the other must be too.
			if (mw.StartDayOfWeek == api.NONE && mw.EndDayOfWeek != api.NONE) || (mw.StartDayOfWeek != api.NONE && mw.EndDayOfWeek == api.NONE) {
				_ = response.BadRequest(errors.New("invalid migration window: both StartDayOfWeek and EndDayOfWeek must be provided")).Render(w)

				return
			}
		}

		// Apply the updated configuration.
		s.state.System.Update.Config = newConfig.Config

		_ = response.EmptySyncResponse.Render(w)

		_ = s.state.Save()
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}
}

func (s *Server) apiSystemUpdateCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Trigger a manual update check.
	s.state.TriggerUpdate <- true

	_ = response.EmptySyncResponse.Render(w)
}
