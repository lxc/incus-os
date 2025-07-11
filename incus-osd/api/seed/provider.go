package seed

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Provider represents the provider seed.
type Provider struct {
	api.SystemProviderConfig `yaml:",inline"`

	Version string `json:"version" yaml:"version"`
}
