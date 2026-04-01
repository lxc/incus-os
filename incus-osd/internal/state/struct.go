package state

import (
	"errors"
	"os"
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
	SuccessfulBoot bool   `json:"successful_boot"`
}

// State represents the on-disk persistent state.
type State struct {
	path string

	StateVersion       int      `json:"-"`
	UnrecognizedFields []string `json:"-"`

	ShouldPerformInstall bool `json:"-"`

	UpdateMutex sync.Mutex `json:"-"`

	JobScheduler scheduling.Scheduler `json:"-"`

	NetworkConfigurationPending bool       `json:"-"`
	NetworkConfigurationChannel chan error `json:"-"`

	// Triggers for daemon actions.
	TriggerReboot   chan bool `json:"-"`
	TriggerShutdown chan bool `json:"-"`
	TriggerSuspend  chan bool `json:"-"`
	TriggerUpdate   chan bool `json:"-"`

	SecureBoot         SecureBoot `json:"secure_boot"`
	UsingSWTPM         bool       `json:"using_swtpm"`
	SecureBootDisabled bool       `json:"secure_boot_disabled"`

	Applications map[string]api.Application `json:"applications"`

	OS OS `json:"os"`

	Services struct {
		Ceph      api.ServiceCeph      `json:"ceph"`
		ISCSI     api.ServiceISCSI     `json:"iscsi"`
		Linstor   api.ServiceLinstor   `json:"linstor"`
		LVM       api.ServiceLVM       `json:"lvm"`
		Multipath api.ServiceMultipath `json:"multipath"`
		Netbird   api.ServiceNetbird   `json:"netbird"`
		NVME      api.ServiceNVME      `json:"nvme"`
		OVN       api.ServiceOVN       `json:"ovn"`
		Tailscale api.ServiceTailscale `json:"tailscale"`
		USBIP     api.ServiceUSBIP     `json:"usbip"`
	} `json:"services"`

	System struct {
		Kernel   api.SystemKernel   `json:"kernel"`
		Logging  api.SystemLogging  `json:"logging"`
		Network  api.SystemNetwork  `json:"network"`
		Provider api.SystemProvider `json:"provider"`
		Security api.SystemSecurity `json:"security"`
		Update   api.SystemUpdate   `json:"update"`
		Storage  api.SystemStorage  `json:"storage"`
	} `json:"system"`

	// Used to handle an edge case of a new network configuration being applied, but
	// the system is rebooted before the new configuration can be confirmed. This helps
	// ensure IncusOS will always be able to boot up with a known good configuration.
	PriorNetworkConfig *api.SystemNetworkConfig `json:"prior_network_config,omitempty"`
}

// MachineID returns the system's persistent machine ID.
func (*State) MachineID() (string, error) {
	machineID, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return "", err
	}

	if len(machineID) != 33 {
		return "", errors.New("invalid length for a machine-id")
	}

	return strings.TrimSpace(string(machineID)), nil
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
	machineID, err := s.MachineID()
	if err == nil {
		// Got what should be a valid UUID, use that.
		return machineID
	}

	// If all else fails, use the OS name.
	return s.OS.Name
}

// RunningFromBackup returns a boolean to indicate if IncusOS is running from
// the older (backup) A/B partition.
func (o *OS) RunningFromBackup() bool {
	return o.NextRelease != "" && o.RunningRelease != o.NextRelease
}
