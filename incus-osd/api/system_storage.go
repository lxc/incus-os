package api

import (
	"encoding/json"
)

// SystemStorage defines a struct to hold information about the system's local storage.
type SystemStorage struct {
	Config struct {
		Pools []SystemStoragePool `json:"pools" yaml:"pools"`
	} `json:"config" yaml:"config"`

	State struct {
		Drives []SystemStorageDrive       `json:"drives" yaml:"drives"`
		Zpools map[string]json.RawMessage `json:"zpools" yaml:"zpools"`
	} `json:"state" yaml:"state"`
}

// SystemStoragePool defines a struct that is used to create or update a ZFS pool.
type SystemStoragePool struct {
	Name string `json:"name" yaml:"name"`
	// Supported pool types: zfs-raid0, zfs-raid1, zfs-raid10, zfs-raidz1, zfs-raidz2, zfs-raidz3.
	Type    string   `json:"type"            yaml:"type"`
	Devices []string `json:"devices"         yaml:"devices"`
	Cache   []string `json:"cache,omitempty" yaml:"cache,omitempty"`
	Log     []string `json:"log,omitempty"   yaml:"log,omitempty"`
}

// SystemStorageDrive defines a struct that holds information about a specific drive.
type SystemStorageDrive struct {
	ID              string                 `json:"id"                yaml:"id"`
	ModelFamily     string                 `json:"model_family"      yaml:"model_family"`
	ModelName       string                 `json:"model_name"        yaml:"model_name"`
	SerialNumber    string                 `json:"serial_number"     yaml:"serial_number"`
	Bus             string                 `json:"bus"               yaml:"bus"`
	CapacityInBytes int                    `json:"capacity_in_bytes" yaml:"capacity_in_bytes"`
	Removable       bool                   `json:"removable"         yaml:"removable"`
	WWN             *SystemStorageDriveWWN `json:"wwn,omitempty"     yaml:"wwn,omitempty"`
	SMARTSupport    json.RawMessage        `json:"smart_support"     yaml:"smart_support"`
	SMARTStatus     json.RawMessage        `json:"smart_status"      yaml:"smart_status"`
	Zpool           string                 `json:"zpool,omitempty"   yaml:"zpool,omitempty"`
}

// SystemStorageDriveWWN defines a struct that holds WWN information.
type SystemStorageDriveWWN struct {
	NAA int `json:"naa" yaml:"naa"`
	OUI int `json:"oui" yaml:"oui"`
	ID  int `json:"id"  yaml:"id"`
}
