package seed

import (
	incusapi "github.com/lxc/incus/v7/shared/api"
)

// Incus represents the Incus seed file.
type Incus struct {
	Version string `json:"version" yaml:"version"`

	ApplyDefaults bool                  `json:"apply_defaults" yaml:"apply_defaults"`
	Preseed       *incusapi.InitPreseed `json:"preseed"        yaml:"preseed"`
}
