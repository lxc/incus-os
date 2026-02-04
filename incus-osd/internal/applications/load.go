package applications

import (
	"context"
	"errors"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// ErrNoPrimary is returned when the system doesn't yet have a primary application.
var ErrNoPrimary = errors.New("no primary application")

// Load retrieves and returns the application specific logic.
func Load(_ context.Context, s *state.State, name string) (Application, error) {
	var app Application

	switch name {
	case "debug":
		app = &debug{common: common{state: s}}
	case "gpu-support":
		app = &gpuSupport{common: common{state: s}}
	case "incus":
		app = &incus{common: common{state: s}}
	case "incus-ceph":
		app = &incusCeph{common: common{state: s}}
	case "incus-linstor":
		app = &incusLinstor{common: common{state: s}}
	case "migration-manager":
		app = &migrationManager{common: common{state: s}}
	case "operations-center":
		app = &operationsCenter{common: common{state: s}}
	default:
		return nil, errors.New("unknown application")
	}

	return app, nil
}

// GetPrimary returns the current primary application once initialized.
func GetPrimary(ctx context.Context, s *state.State) (Application, error) {
	for appName, v := range s.Applications {
		// Skip uninitialized applications.
		if !v.State.Initialized {
			continue
		}

		// Load the application.
		app, err := Load(ctx, s, appName)
		if err != nil {
			return nil, err
		}

		if app.IsPrimary() {
			return app, nil
		}
	}

	return nil, ErrNoPrimary
}
