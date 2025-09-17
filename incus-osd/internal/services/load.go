package services

import (
	"context"
	"errors"
	"slices"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Supported returns the list of all valid services for this system.
// The list is sorted in recommended startup order to handle service dependencies.
func Supported(s *state.State) []string {
	services := []string{"ceph", "iscsi", "nvme", "multipath", "lvm", "ovn", "usbip"}
	supported := make([]string, 0, len(services))

	for _, service := range services {
		srv, err := loadByName(s, service)
		if err != nil {
			continue
		}

		if !srv.Supported() {
			continue
		}

		supported = append(supported, service)
	}

	return supported
}

// Load returns a handler for the given system service.
func Load(_ context.Context, s *state.State, name string) (Service, error) {
	if !slices.Contains(Supported(s), name) {
		return nil, errors.New("unknown service")
	}

	// Load the service.
	srv, err := loadByName(s, name)
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func loadByName(s *state.State, name string) (Service, error) {
	var srv Service

	switch name {
	case "ceph":
		srv = &Ceph{state: s}
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

	return srv, nil
}
