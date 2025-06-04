package state

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Application represents an installed application (system extension).
type Application struct {
	Initialized bool   `json:"initialized"`
	Version     string `json:"version"`
}

// OS represents the current OS image state.
type OS struct {
	Name           string `json:"name"`
	RunningRelease string `json:"running_release"`
	NextRelease    string `json:"next_release"`
}

// State represents the on-disk persistent state.
type State struct {
	path string

	ShouldPerformInstall bool `json:"-"`

	// Triggers for daemon actions.
	TriggerReboot   chan error `json:"-"`
	TriggerShutdown chan error `json:"-"`
	TriggerUpdate   chan bool  `json:"-"`

	Applications map[string]Application `json:"applications"`

	OS OS `json:"os"`

	Services struct {
		ISCSI api.ServiceISCSI `json:"iscsi"`
		LVM   api.ServiceLVM   `json:"lvm"`
		NVME  api.ServiceNVME  `json:"nvme"`
		OVN   api.ServiceOVN   `json:"ovn"`
	} `json:"services"`

	System struct {
		Encryption api.SystemEncryption `json:"encryption"`
		Network    api.SystemNetwork    `json:"network"`
		Provider   api.SystemProvider   `json:"provider"`
	} `json:"system"`
}
