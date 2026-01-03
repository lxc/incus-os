package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/scheduling"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
	"github.com/lxc/incus-os/incus-osd/internal/zfs"
)

// swagger:operation GET /1.0/system/storage system system_get_storage
//
//	Get storage information
//
//	Returns information about drives present in the system and the status of any local storage pools.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for the system storage
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
//	          description: State and configuration for the system storage
//	          example: {"config":{},"state":{"drives":[{"id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root","model_family":"QEMU","model_name":"QEMU HARDDISK","serial_number":"incus_root","bus":"scsi","capacity_in_bytes":53687091200,"boot":true,"removable":false,"remote":false}],"pools":[{"name":"local","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11"],"state":"ONLINE","encryption_key_status":"available","raw_pool_size_in_bytes":17716740096,"usable_pool_size_in_bytes":17716740096,"pool_allocated_space_in_bytes":4313088}]}}
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation PUT /1.0/system/storage system system_put_storage
//
//	Update system storage configuration
//
//	Creates or updates a local storage pool.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Storage configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The storage configuration
//	          example: {"pools":[{"name":"mypool","type":"zfs-raidz3","devices":["/dev/sdb","/dev/sdc","/dev/sdd","/dev/sde"]}]}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemStorage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		ret, err := storage.GetStorageInfo(r.Context())
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Populate config with the current system config.
		ret.Config = s.state.System.Storage.Config

		// Return the current system storage state.
		_ = response.SyncResponse(true, ret).Render(w)
	case http.MethodPut:
		// Ensure any state updates are persisted.
		defer s.state.Save()

		// Get the current configuration.
		current, err := storage.GetStorageInfo(r.Context())
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Read the new config.
		storageStruct := &api.SystemStorage{}

		counter := &countWrapper{ReadCloser: r.Body}

		err = json.NewDecoder(counter).Decode(storageStruct)
		if err != nil && counter.n > 0 {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Apply new schedule to the job scheduler.
		err = s.state.JobScheduler.RegisterJob(zfs.PoolScrubJob, storageStruct.Config.ScrubSchedule, zfs.ScrubAllPools)
		if err != nil {
			if errors.Is(err, scheduling.ErrInvalidCronTab) {
				_ = response.BadRequest(errors.New("invalid cron expression provided for scrub schedule")).Render(w)

				return
			}

			_ = response.InternalError(err).Render(w)

			return
		}

		// Update scrub schedule in state.
		s.state.System.Storage.Config.ScrubSchedule = storageStruct.Config.ScrubSchedule

		// Create or update a pool.
		if len(storageStruct.Config.Pools) == 0 {
			if len(current.Config.Pools) == 0 {
				_ = response.EmptySyncResponse.Render(w)

				return
			}

			_ = response.BadRequest(errors.New("no pool configuration provided")).Render(w)

			return
		}

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
}

// swagger:operation POST /1.0/system/storage/:delete-pool system system_post_storage_delete_pool
//
//	Delete local pool
//
//	Destroys a local storage pool.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: The pool to be deleted
//	    required: true
//	    schema:
//	      type: object
//	      example: {"name":"mypool"}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiSystemStorageDeletePool(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	type deleteStruct struct {
		Name string `json:"name"`
	}

	config := &deleteStruct{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(config)
	if err != nil && counter.n > 0 {
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

// swagger:operation POST /1.0/system/storage/:wipe-drive system system_post_storage_wipe_drive
//
//	Wipe a drive
//
//	Forcibly wipes all data from the specified drive.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: The drive to be wiped
//	    required: true
//	    schema:
//	      type: object
//	      example: {"id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk"}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiSystemStorageWipeDrive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Wipe the specified drive.
	wipeStruct := &api.SystemStorageWipe{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(wipeStruct)
	if err != nil && counter.n > 0 {
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

// swagger:operation POST /1.0/system/storage/:import-pool system system_post_storage_import_pool
//
//	Import an existing encrypted storage pool
//
//	Imports an existing encrypted ZFS storage pool and save its encryption key.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Existing pool information
//	    required: true
//	    schema:
//	      type: object
//	      example: {"name":"mypool","type":"zfs","encryption_key":"THp6YZ33zwAEXiCWU71/l7tY8uWouKB5TSr/uKXCj2A="}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiSystemStorageImportPool(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	// Add the specified encryption key for the manually imported pool.
	poolStruct := &api.SystemStoragePoolKey{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(poolStruct)
	if err != nil && counter.n > 0 {
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

	err = zfs.ImportExistingPool(r.Context(), poolStruct.Name, poolStruct.EncryptionKey)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/system/storage/:create-volume system system_post_storage_create_volume
//
//	Create a volume
//
//	Creates a new storage pool volume.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: The volume to be created
//	    required: true
//	    schema:
//	      type: object
//	      example: {"pool":"local", "name":"my-volume", "quota":0, "use":"incus"}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiSystemStorageCreateVolume(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	type createStruct struct {
		Pool  string `json:"pool"`
		Name  string `json:"name"`
		Quota int    `json:"quota"`
		Use   string `json:"use"`
	}

	config := &createStruct{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(config)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if config.Pool == "" {
		_ = response.BadRequest(errors.New("no pool name provided")).Render(w)

		return
	}

	if strings.Contains(config.Pool, "/") {
		_ = response.BadRequest(errors.New("invalid pool name provided")).Render(w)

		return
	}

	if config.Name == "" {
		_ = response.BadRequest(errors.New("no volume name provided")).Render(w)

		return
	}

	if strings.Contains(config.Name, "/") {
		_ = response.BadRequest(errors.New("invalid volume name provided")).Render(w)

		return
	}

	if config.Use == "" {
		_ = response.BadRequest(errors.New("no volume use provided")).Render(w)

		return
	}

	if !slices.Contains([]string{"incus", "linstor"}, config.Use) {
		_ = response.BadRequest(errors.New("invalid volume use provided")).Render(w)

		return
	}

	// Create the volume.
	props := map[string]string{}
	props["incusos:use"] = config.Use

	if config.Use == "linstor" {
		// Linstor doesn't support encryption.
		props["encryption"] = "off"
	}

	if config.Quota > 0 {
		props["quota"] = strconv.Itoa(config.Quota)
	}

	err = zfs.CreateDataset(r.Context(), config.Pool, config.Name, props)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/system/storage/:delete-volume system system_post_storage_delete_volume
//
//	Delete a volume
//
//	Deletes a storage pool volume.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: The volume to be deleted
//	    required: true
//	    schema:
//	      type: object
//	      example: {"pool":"local", "name":"my-volume", "force":true}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiSystemStorageDeleteVolume(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	type deleteStruct struct {
		Pool  string `json:"pool"`
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}

	config := &deleteStruct{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(config)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if config.Pool == "" {
		_ = response.BadRequest(errors.New("no pool name provided")).Render(w)

		return
	}

	if strings.Contains(config.Pool, "/") {
		_ = response.BadRequest(errors.New("invalid pool name provided")).Render(w)

		return
	}

	if config.Name == "" {
		_ = response.BadRequest(errors.New("no volume name provided")).Render(w)

		return
	}

	if strings.Contains(config.Name, "/") {
		_ = response.BadRequest(errors.New("invalid volume name provided")).Render(w)

		return
	}

	// Delete the volume.
	err = zfs.DestroyDataset(r.Context(), config.Pool, config.Name, config.Force)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}

// swagger:operation POST /1.0/system/storage/:scrub-pool system system_post_storage_scrub_pool
//
//	Scrub local pool
//
//	Starts a scrub for a local storage pool.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: The pool to be scrubbed
//	    required: true
//	    schema:
//	      type: object
//	      example: {"name":"mypool"}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "409":
//	    $ref: "#/responses/Conflict"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiSystemStorageScrubPool(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	type scrubStruct struct {
		Name string `json:"name"`
	}

	config := &scrubStruct{}

	counter := &countWrapper{ReadCloser: r.Body}

	err := json.NewDecoder(counter).Decode(config)
	if err != nil && counter.n > 0 {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if config.Name == "" {
		_ = response.BadRequest(errors.New("no pool name provided")).Render(w)

		return
	}

	// Scrub the pool.
	err = zfs.ScrubZpool(r.Context(), config.Name)
	if err != nil {
		if errors.Is(err, storage.ErrScrubAlreadyInProgress) {
			_ = response.Conflict(err).Render(w)
		} else {
			_ = response.InternalError(err).Render(w)
		}

		return
	}

	_ = response.EmptySyncResponse.Render(w)
}
