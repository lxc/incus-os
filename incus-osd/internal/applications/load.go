package applications

import (
	"context"
)

// Load retrieves and returns the application specific logic.
func Load(_ context.Context, name string) (Application, error) {
	var app Application

	switch name {
	case "incus":
		app = &incus{}
	case "migration-manager":
		app = &migrationManager{}
	default:
		app = &common{}
	}

	return app, nil
}
