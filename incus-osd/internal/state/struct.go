package state

import (
	"os"
	"strings"

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
		USBIP api.ServiceUSBIP `json:"usbip"`
	} `json:"services"`

	System struct {
		Encryption api.SystemEncryption `json:"encryption"`
		Network    api.SystemNetwork    `json:"network"`
		Provider   api.SystemProvider   `json:"provider"`
	} `json:"system"`
}

// Hostname returns the preferred hostname for the system.
func (s *State) Hostname() string {
	// Use the configured hostname if set by the user.
	if s.System.Network.Config != nil && s.System.Network.Config.DNS != nil && s.System.Network.Config.DNS.Hostname != "" {
		hostname := s.System.Network.Config.DNS.Hostname
		if s.System.Network.Config.DNS.Domain != "" {
			hostname += "." + s.System.Network.Config.DNS.Domain
		}

		return hostname
	}

	// Use product UUID if valid.
	productUUID, err := os.ReadFile("/sys/class/dmi/id/product_uuid")
	if err == nil && len(productUUID) == 37 {
		// Got what should be a valid UUID, use that.
		return strings.TrimSpace(string(productUUID))
	}

	// Use machine ID if valid.
	machineID, err := os.ReadFile("/etc/machine-id")
	if err == nil && len(machineID) == 33 {
		// Got what should be a valid UUID, use that.
		return strings.TrimSpace(string(machineID))
	}

	// If all else fails, use the OS name.
	return s.OS.Name
}
