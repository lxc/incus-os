package seed

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Security represents the security seed.
type Security struct {
	api.SystemSecurityConfig `yaml:",inline"`

	Version string `json:"version" yaml:"version"`
}
