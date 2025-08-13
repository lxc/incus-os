package services

import (
	"context"
	"errors"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Supported contains the list of all valid services.
// The list is sorted in recommended startup order to handle service dependencies.
var Supported = []string{"iscsi", "nvme", "multipath", "lvm", "ovn", "usbip"}

// Load returns a handler for the given system service.
func Load(ctx context.Context, s *state.State, name string) (Service, error) {
	var srv Service

	switch name {
	case "iscsi":
		srv = &ISCSI{state: s}
	case "lvm":
		srv = &LVM{state: s}
	case "multipath":
		srv = &Multipath{state: s}
	case "nvme":
		srv = &NVME{state: s}
	case "ovn":
		srv = &OVN{state: s}
	case "usbip":
		srv = &USBIP{state: s}
	default:
		return nil, errors.New("unknown service")
	}

	// Initialize the service.
	err := srv.init(ctx)
	if err != nil {
		return nil, err
	}

	return srv, nil
}
