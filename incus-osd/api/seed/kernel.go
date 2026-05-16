package seed

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Kernel represents the kernel seed.
type Kernel struct {
	Console []api.SystemKernelConfigConsole `json:"console,omitempty" yaml:"console,omitempty"`

	Version string `json:"version" yaml:"version"`
}
