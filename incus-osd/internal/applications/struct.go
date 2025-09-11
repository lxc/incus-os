package applications

import (
	"context"
)

// Application represents an installed application.
type Application interface {
	Start(ctx context.Context, version string) error
	Stop(ctx context.Context, version string) error
	InitializePreStart(ctx context.Context) error
	InitializePostStart(ctx context.Context) error
	Update(ctx context.Context, version string) error
	IsRunning(ctx context.Context) bool
}
