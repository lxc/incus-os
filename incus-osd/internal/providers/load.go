package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Load gets a specific provider and initializes it with the provider configuration.
func Load(ctx context.Context, s *state.State) (Provider, error) {
	// NOTE: Migration logic, remove after a few releases.
	if s.System.Provider.Config.Name == "github" {
		s.System.Provider.Config.Name = "images"
	}

	var p Provider

	switch s.System.Provider.Config.Name {
	case "images":
		// Setup the images provider.
		p = &images{
			state: s,
		}

	case "local":
		// Setup the local provider.
		p = &local{
			state: s,
		}

	case "operations-center":
		// Setup the Operations Center provider.
		p = &operationsCenter{
			state: s,
		}

	default:
		return nil, fmt.Errorf("unknown provider %q", s.System.Provider.Config.Name)
	}

	err := p.load(ctx)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Refresh is a hook being called whenever the current provider should be refreshed.
func Refresh(ctx context.Context, s *state.State) error {
	if s.System.Provider.Config.Name == "" {
		return nil
	}

	p, err := Load(ctx, s)
	if err != nil {
		return err
	}

	err = p.RefreshRegister(ctx)
	if err != nil && !errors.Is(err, ErrRegistrationUnsupported) {
		return err
	}

	return nil
}
