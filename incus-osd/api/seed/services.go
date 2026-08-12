package seed

import (
	"github.com/lxc/incus-os/incus-osd/api"
)

// Services represents the services seed.
type Services struct {
	ISCSI     *api.ServiceISCSIConfig     `json:"iscsi,omitempty"     yaml:"iscsi,omitempty"`
	LVM       *api.ServiceLVMConfig       `json:"lvm,omitempty"       yaml:"lvm,omitempty"`
	Multipath *api.ServiceMultipathConfig `json:"multipath,omitempty" yaml:"multipath,omitempty"`
	Netbird   *api.ServiceNetbirdConfig   `json:"netbird,omitempty"   yaml:"netbird,omitempty"`
	NVME      *api.ServiceNVMEConfig      `json:"nvme,omitempty"      yaml:"nvme,omitempty"`
	OVN       *api.ServiceOVNConfig       `json:"ovn,omitempty"       yaml:"ovn,omitempty"`
	Tailscale *api.ServiceTailscaleConfig `json:"tailscale,omitempty" yaml:"tailscale,omitempty"`
	USBIP     *api.ServiceUSBIPConfig     `json:"usbip,omitempty"     yaml:"usbip,omitempty"`

	Version string `json:"version" yaml:"version"`
}
