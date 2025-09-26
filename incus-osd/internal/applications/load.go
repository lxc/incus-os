package applications

import (
	"context"
	"errors"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// ErrNoPrimary is returned when the system doesn't yet have a primary application.
var ErrNoPrimary = errors.New("no primary application")

// Load retrieves and returns the application specific logic.
func Load(_ context.Context, name string) (Application, error) {
	var app Application

	switch name {
	case "incus":
		app = &incus{}
	case "migration-manager":
		app = &migrationManager{}
	case "operations-center":
		app = &operationsCenter{}
	default:
		app = &common{}
	}

	return app, nil
}

// GetPrimary returns the current primary application.
func GetPrimary(ctx context.Context, s *state.State) (Application, error) {
	for appName := range s.Applications {
		app, err := Load(ctx, appName)
		if err != nil {
			return nil, err
		}

		if app.IsPrimary() {
			return app, nil
		}
	}

	return nil, ErrNoPrimary
}
