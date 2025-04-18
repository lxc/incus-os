package applications

import (
	"context"
)

// Application represents an installed application.
type Application interface {
	Start(ctx context.Context, version string) error
	Stop(ctx context.Context, version string) error
	Initialize(ctx context.Context) error
}
