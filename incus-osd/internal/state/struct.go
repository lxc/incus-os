package state

import (
	"net"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/scheduling"
)

// SecureBoot represents the current state of Secure Boot key updates applied to the system.
type SecureBoot struct {
	Version      string `json:"version"`
	FullyApplied bool   `json:"fully_applied"`
}

// OS represents the current OS image state.
type OS struct {
	Name           string `json:"name"`
	RunningRelease string `json:"running_release"`
	NextRelease    string `json:"next_release"`
	SuccessfulBoot bool   `jsno:"successful_boot"`
}

// State represents the on-disk persistent state.
type State struct {
	path string

	StateVersion       int      `json:"-"`
	UnrecognizedFields []string `json:"-"`

	ShouldPerformInstall bool `json:"-"`

	UpdateMutex sync.Mutex `json:"-"`

	JobScheduler scheduling.Scheduler `json:"-"`

	// Triggers for daemon actions.
	TriggerReboot   chan error `json:"-"`
	TriggerShutdown chan error `json:"-"`
	TriggerUpdate   chan bool  `json:"-"`

	SecureBoot SecureBoot `json:"secure_boot"`
	UsingSWTPM bool       `json:"using_swtpm"`

	Applications map[string]api.Application `json:"applications"`

	OS OS `json:"os"`

	Services struct {
		Ceph      api.ServiceCeph      `json:"ceph"`
		ISCSI     api.ServiceISCSI     `json:"iscsi"`
		Linstor   api.ServiceLinstor   `json:"linstor"`
		LVM       api.ServiceLVM       `json:"lvm"`
		Multipath api.ServiceMultipath `json:"multipath"`
		NVME      api.ServiceNVME      `json:"nvme"`
		OVN       api.ServiceOVN       `json:"ovn"`
		Tailscale api.ServiceTailscale `json:"tailscale"`
		USBIP     api.ServiceUSBIP     `json:"usbip"`
	} `json:"services"`

	System struct {
		Logging  api.SystemLogging  `json:"logging"`
		Network  api.SystemNetwork  `json:"network"`
		Provider api.SystemProvider `json:"provider"`
		Security api.SystemSecurity `json:"security"`
		Update   api.SystemUpdate   `json:"update"`
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

// ManagementAddress returns the preferred IP address at which to reach this server for management purposes.
// A nil value is returned if none could be found.
func (s *State) ManagementAddress() net.IP {
	if len(s.System.Network.State.Interfaces) == 0 {
		return nil
	}

	var (
		ipv4Address net.IP
		ipv6Address net.IP
	)

	for _, iface := range s.System.Network.State.Interfaces {
		// Skip if missing management role.
		if !slices.Contains(iface.Roles, api.SystemNetworkInterfaceRoleManagement) {
			continue
		}

		for _, address := range iface.Addresses {
			addrIP := net.ParseIP(address)
			if addrIP == nil {
				continue
			}

			if addrIP.To4() == nil {
				if ipv6Address == nil {
					ipv6Address = addrIP
				}
			} else {
				if ipv4Address == nil {
					ipv4Address = addrIP
				}
			}
		}

		// Break early if we have an IPv6 address as we'll prefer that anyway.
		if ipv6Address != nil {
			break
		}
	}

	if ipv6Address != nil {
		return ipv6Address
	}

	if ipv4Address != nil {
		return ipv4Address
	}

	return nil
}
