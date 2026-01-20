package api

import "time"

// SystemStorageConfig represents additional configuration for the system's local storage.
type SystemStorageConfig struct {
	ScrubSchedule string              `json:"scrub_schedule" yaml:"scrub_schedule"`
	Pools         []SystemStoragePool `incusos:"-"           json:"pools,omitempty" yaml:"pools,omitempty"`
}

// SystemStorageState represents additional state for the system's local storage.
type SystemStorageState struct {
	Drives []SystemStorageDrive `json:"drives" yaml:"drives"`
	Pools  []SystemStoragePool  `json:"pools"  yaml:"pools"`
}

// SystemStorage defines a struct to hold information about the system's local storage.
type SystemStorage struct {
	Config SystemStorageConfig `json:"config" yaml:"config"`

	State SystemStorageState `incusos:"-" json:"state" yaml:"state"`
}

// SystemStoragePool defines a struct that is used to create or update a storage pool and return its current state.
type SystemStoragePool struct {
	// Name and Type cannot be changed after pool creation.
	Name string `json:"name" yaml:"name"`
	// Supported pool types: zfs-raid0, zfs-raid1, zfs-raid10, zfs-raidz1, zfs-raidz2, zfs-raidz3.
	Type string `json:"type" yaml:"type"`

	// Devices, Cache, and Log can be modified to add/remove/replace devices in the pool.
	Devices []string `json:"devices"         yaml:"devices"`
	Cache   []string `json:"cache,omitempty" yaml:"cache,omitempty"`
	Log     []string `json:"log,omitempty"   yaml:"log,omitempty"`

	// Read-only fields returned from the server with additional pool information.
	State                     string                        `json:"state"                         yaml:"state"`
	LastScrub                 *SystemStoragePoolScrubStatus `json:"last_scrub,omitempty"          yaml:"last_scrub,omitempty,omitempty"`
	EncryptionKeyStatus       string                        `json:"encryption_key_status"         yaml:"encryption_key_status"`
	DevicesDegraded           []string                      `json:"devices_degraded,omitempty"    yaml:"devices_degraded,omitempty"`
	CacheDegraded             []string                      `json:"cache_degraded,omitempty"      yaml:"cache_degraded,omitempty"`
	LogDegraded               []string                      `json:"log_degraded,omitempty"        yaml:"log_degraded,omitempty"`
	RawPoolSizeInBytes        int                           `json:"raw_pool_size_in_bytes"        yaml:"raw_pool_size_in_bytes"`
	UsablePoolSizeInBytes     int                           `json:"usable_pool_size_in_bytes"     yaml:"usable_pool_size_in_bytes"`
	PoolAllocatedSpaceInBytes int                           `json:"pool_allocated_space_in_bytes" yaml:"pool_allocated_space_in_bytes"`
	Volumes                   []SystemStoragePoolVolume     `json:"volumes"                       yaml:"volumes"`
}

// SystemStoragePoolVolume represents a single IncusOS-managed volume in a pool.
type SystemStoragePoolVolume struct {
	Name         string `json:"name"           yaml:"name"`
	UsageInBytes int    `json:"usage_in_bytes" yaml:"usage_in_bytes"`
	QuotaInBytes int    `json:"quota_in_bytes" yaml:"quota_in_bytes"`
	Use          string `json:"use"            yaml:"use"`
}

// SystemStoragePoolScrubState represents the state of a scan in a pool.
type SystemStoragePoolScrubState string

const (
	// ScrubUnknown represents and unknown scrub status.
	ScrubUnknown SystemStoragePoolScrubState = "UNKNOWN"
	// ScrubInProgress represents that the scrub is in progress.
	ScrubInProgress SystemStoragePoolScrubState = "IN_PROGRESS"
	// ScrubFinished represents that the scrub has finished.
	ScrubFinished SystemStoragePoolScrubState = "FINISHED"
)

// SystemStoragePoolScrubStatus represents the status of a scrub in a pool.
type SystemStoragePoolScrubStatus struct {
	State     SystemStoragePoolScrubState `json:"state"`
	StartTime time.Time                   `json:"start_time"`
	EndTime   time.Time                   `json:"end_time"`
	Progress  string                      `json:"progress"`
	Errors    int                         `json:"errors"`
}

// SystemStorageDrive defines a struct that holds information about a specific drive.
type SystemStorageDrive struct {
	ID              string                   `json:"id"                    yaml:"id"`
	ModelFamily     string                   `json:"model_family"          yaml:"model_family"`
	ModelName       string                   `json:"model_name"            yaml:"model_name"`
	SerialNumber    string                   `json:"serial_number"         yaml:"serial_number"`
	Bus             string                   `json:"bus"                   yaml:"bus"`
	CapacityInBytes int                      `json:"capacity_in_bytes"     yaml:"capacity_in_bytes"`
	Boot            bool                     `json:"boot"                  yaml:"boot"`
	Removable       bool                     `json:"removable"             yaml:"removable"`
	Remote          bool                     `json:"remote"                yaml:"remote"`
	WWN             string                   `json:"wwn,omitempty"         yaml:"wwn,omitempty"`
	SMART           *SystemStorageDriveSMART `json:"smart,omitempty"       yaml:"smart,omitempty"`
	MemberPool      string                   `json:"member_pool,omitempty" yaml:"member_pool,omitempty"`
}

// SystemStorageDriveSMART defines a struct to return basic SMART information about a specific device.
type SystemStorageDriveSMART struct {
	Enabled bool   `json:"enabled"         yaml:"enabled"`
	Passed  bool   `json:"passed"          yaml:"passed"`
	Error   string `json:"error,omitempty" yaml:"error,omitempty"`

	PowerOnHours       int `json:"power_on_hours,omitempty"      yaml:"power_on_hours,omitempty"`
	DataUnitsRead      int `json:"data_units_read,omitempty"     yaml:"data_units_read,omitempty"`
	DataUnitsWritten   int `json:"data_units_written,omitempty"  yaml:"data_units_written,omitempty"`
	AvailableSpare     int `json:"available_spare,omitempty"     yaml:"available_spare,omitempty"`
	PercentageUsed     int `json:"percentage_used,omitempty"     yaml:"percentage_used,omitempty"`
	RawReadErrorRate   int `json:"raw_read_error_rate,omitempty" yaml:"raw_read_error_rate,omitempty"`
	SeekErrorRate      int `json:"seek_error_rate,omitempty"     yaml:"seek_error_rate,omitempty"`
	ReallocatedSectors int `json:"reallocated_sectors,omitempty" yaml:"reallocated_sectors,omitempty"`
}

// SystemStorageWipe defines a struct with information about what drive to wipe.
type SystemStorageWipe struct {
	ID string `json:"id" yaml:"id"`
}

// SystemStoragePoolKey defines a struct used to provide an encryption key when importing an existing pool.
// Currently the only supported type is "zfs".
type SystemStoragePoolKey struct {
	Name          string `json:"name"           yaml:"name"`
	Type          string `json:"type"           yaml:"type"`
	EncryptionKey string `json:"encryption_key" yaml:"encryption_key"`
}
