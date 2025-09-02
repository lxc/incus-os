package rest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
	"github.com/lxc/incus-os/incus-osd/internal/zfs"
)

func (s *Server) apiSystemStorage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		ret, err := storage.GetStorageInfo(r.Context())
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Return the current system storage state.
		_ = response.SyncResponse(true, ret).Render(w)
	case http.MethodPut, http.MethodDelete:
		// Create, update, or delete a ZFS pool.
		if r.ContentLength <= 0 {
			_ = response.BadRequest(errors.New("no ZFS pool configuration provided")).Render(w)

			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		storageStruct := api.SystemStorage{}

		err = json.Unmarshal(b, &storageStruct)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		if len(storageStruct.Config.Pools) == 0 {
			_ = response.BadRequest(errors.New("no ZFS pool configuration provided")).Render(w)

			return
		}

		if r.Method == http.MethodPut {
			// Create or update a pool.
			for _, pool := range storageStruct.Config.Pools {
				if !zfs.PoolExists(r.Context(), pool.Name) {
					err = zfs.CreateZpool(r.Context(), pool, s.state)
				} else {
					err = zfs.UpdateZpool(r.Context(), pool)
				}

				if err != nil {
					_ = response.BadRequest(err).Render(w)

					return
				}
			}
		} else {
			// Delete a pool.
			for _, pool := range storageStruct.Config.Pools {
				err = zfs.DestroyZpool(r.Context(), pool.Name)
				if err != nil {
					_ = response.BadRequest(err).Render(w)

					return
				}
			}
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save(r.Context())
}

func (*Server) apiSystemStorageWipe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		// Wipe the specified drive.
		if r.ContentLength <= 0 {
			_ = response.BadRequest(errors.New("no drive specified")).Render(w)

			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		wipeStruct := api.SystemStorageWipe{}

		err = json.Unmarshal(b, &wipeStruct)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		if wipeStruct.ID == "" {
			_ = response.BadRequest(errors.New("no drive specified")).Render(w)

			return
		}

		err = storage.WipeDrive(r.Context(), wipeStruct.ID)
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
