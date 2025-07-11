package seed

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Network represents the network seed.
type Network struct {
	api.SystemNetworkConfig `yaml:",inline"`

	Version string `json:"version" yaml:"version"`
}
