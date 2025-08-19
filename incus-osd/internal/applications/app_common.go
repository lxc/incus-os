package applications

import (
	"context"
)

type common struct{}

// InitializePreStart runs first time initialization before the application starts.
func (*common) InitializePreStart(_ context.Context) error {
	return nil
}

// InitializePostStart runs first time initialization after the application starts.
func (*common) InitializePostStart(_ context.Context) error {
	return nil
}

// Start runs startup action.
func (*common) Start(_ context.Context, _ string) error {
	return nil
}

// Stop runs shutdown action.
func (*common) Stop(_ context.Context, _ string) error {
	return nil
}

// Update triggers a partial application restart after an update.
func (*common) Update(_ context.Context, _ string) error {
	return nil
}

// IsRunning reports if the application is currently running.
func (*common) IsRunning(_ context.Context) bool {
	return true
}
