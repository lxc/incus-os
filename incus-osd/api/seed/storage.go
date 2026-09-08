package seed

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Storage represents the storage seed used to automatically
// create storage pools on first boot.
type Storage struct {
	Pools []api.SystemStoragePool `json:"pools,omitempty" yaml:"pools,omitempty"`

	Version string `json:"version" yaml:"version"`
}
