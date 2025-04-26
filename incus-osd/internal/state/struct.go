package state

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Application represents an installed application (system extension).
type Application struct {
	Initialized bool   `json:"initialized"`
	Version     string `json:"version"`
}

// State represents the on-disk persistent state.
type State struct {
	path string

	// Triggers for daemon shutdown sequence.
	TriggerReboot   chan error `json:"-"`
	TriggerShutdown chan error `json:"-"`

	Applications   map[string]Application `json:"applications"`
	RunningRelease string                 `json:"running_release"`

	Services struct {
		ISCSI api.ServiceISCSI `json:"iscsi"`
		LVM   api.ServiceLVM   `json:"lvm"`
		NVME  api.ServiceNVME  `json:"nvme"`
		OVN   api.ServiceOVN   `json:"ovn"`
	} `json:"services"`

	System struct {
		Encryption api.SystemEncryption `json:"encryption"`
		Network    api.SystemNetwork    `json:"network"`
	} `json:"system"`
}
