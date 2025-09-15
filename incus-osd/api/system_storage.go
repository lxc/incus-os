package api

// SystemStorage defines a struct to hold information about the system's local storage.
type SystemStorage struct {
	Config struct {
		Pools []SystemStoragePool `json:"pools,omitempty" yaml:"pools,omitempty"`
	} `json:"config" yaml:"config"`

	State struct {
		Drives []SystemStorageDrive `json:"drives" yaml:"drives"`
		Pools  []SystemStoragePool  `json:"pools"  yaml:"pools"`
	} `json:"state" yaml:"state"`
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
	State                     string   `json:"state"                         yaml:"state"`
	DevicesDegraded           []string `json:"devices_degraded,omitempty"    yaml:"devices_degraded,omitempty"`
	CacheDegraded             []string `json:"cache_degraded,omitempty"      yaml:"cache_degraded,omitempty"`
	LogDegraded               []string `json:"log_degraded,omitempty"        yaml:"log_degraded,omitempty"`
	RawPoolSizeInBytes        int      `json:"raw_pool_size_in_bytes"        yaml:"raw_pool_size_in_bytes"`
	UsablePoolSizeInBytes     int      `json:"usable_pool_size_in_bytes"     yaml:"usable_pool_size_in_bytes"`
	PoolAllocatedSpaceInBytes int      `json:"pool_allocated_space_in_bytes" yaml:"pool_allocated_space_in_bytes"`
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
	Enabled bool `json:"enabled" yaml:"enabled"`
	Passed  bool `json:"passed"  yaml:"passed"`
}

// SystemStorageWipe defines a struct with information about what drive to wipe.
type SystemStorageWipe struct {
	ID string `json:"id" yaml:"id"`
}
