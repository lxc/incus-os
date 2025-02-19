package providers

import (
	"context"
)

// Application represents an application to be installed on top of Incus OS.
type Application interface {
	Name() string
	Version() string

	Download(ctx context.Context, targetPath string) error
}

// OSUpdate represents a full OS update.
type OSUpdate interface {
	Version() string

	Download(ctx context.Context, targetPath string) error
}

// Provider represents an update/application provider.
type Provider interface {
	GetOSUpdate(ctx context.Context) (OSUpdate, error)
	GetApplication(ctx context.Context, name string) (Application, error)

	load(ctx context.Context) error
}
