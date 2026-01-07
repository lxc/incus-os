package seed

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Update represents the update seed.
type Update struct {
	api.SystemUpdateConfig `yaml:",inline"`

	Version string `json:"version" yaml:"version"`
}
