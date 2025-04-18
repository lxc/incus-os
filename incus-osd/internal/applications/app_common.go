package applications

import (
	"context"
)

type common struct{}

// Start runs startup action.
func (*common) Start(_ context.Context, _ string) error {
	return nil
}

// Stop runs shutdown action.
func (*common) Stop(_ context.Context, _ string) error {
	return nil
}

// Initialize runs first time initialization.
func (*common) Initialize(_ context.Context) error {
	return nil
}
