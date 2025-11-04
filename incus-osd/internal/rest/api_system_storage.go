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
			_ = response.InternalError(err).Render(w)

			return
		}

		// Return the current system storage state.
		_ = response.SyncResponse(true, ret).Render(w)
	case http.MethodPut:
		// Create or update a pool.
		if r.ContentLength <= 0 {
			_ = response.BadRequest(errors.New("no pool configuration provided")).Render(w)

			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		storageStruct := api.SystemStorage{}

		err = json.Unmarshal(b, &storageStruct)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		if len(storageStruct.Config.Pools) == 0 {
			_ = response.BadRequest(errors.New("no pool configuration provided")).Render(w)

			return
		}

		// Create or update a pool.
		for _, pool := range storageStruct.Config.Pools {
			if !storage.PoolExists(r.Context(), pool.Name) {
				err = zfs.CreateZpool(r.Context(), pool, s.state)
			} else {
				err = zfs.UpdateZpool(r.Context(), pool)
			}

			if err != nil {
				_ = response.InternalError(err).Render(w)

				return
			}
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save()
}

func (*Server) apiSystemStorageDeletePool(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	if r.ContentLength <= 0 {
		_ = response.BadRequest(errors.New("no pool configuration provided")).Render(w)

		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	type deleteStruct struct {
		Name string `json:"name"`
	}

	config := deleteStruct{}

	err = json.Unmarshal(b, &config)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if config.Name == "" {
		_ = response.BadRequest(errors.New("no pool name provided")).Render(w)

		return
	}

	// Delete the pool.
	err = zfs.DestroyZpool(r.Context(), config.Name)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

func (*Server) apiSystemStorageWipeDrive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Wipe the specified drive.
	if r.ContentLength <= 0 {
		_ = response.BadRequest(errors.New("no drive specified")).Render(w)

		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		_ = response.InternalError(err).Render(w)

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
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

func (*Server) apiSystemStorageImportEncryptionKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Add the specified encryption key for the manually imported pool.
	if r.ContentLength <= 0 {
		_ = response.BadRequest(errors.New("no pool configuration specified")).Render(w)

		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	poolStruct := api.SystemStoragePoolKey{}

	err = json.Unmarshal(b, &poolStruct)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if poolStruct.Name == "" || poolStruct.Type == "" || poolStruct.EncryptionKey == "" {
		_ = response.BadRequest(errors.New("missing pool name, type, and/or encryption key")).Render(w)

		return
	}

	if poolStruct.Type != "zfs" {
		_ = response.BadRequest(errors.New("unsupported pool type '" + poolStruct.Type + "'")).Render(w)

		return
	}

	err = storage.SetEncryptionKey(r.Context(), poolStruct.Name, poolStruct.EncryptionKey)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}
