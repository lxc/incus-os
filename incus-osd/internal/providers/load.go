package providers

import (
	"context"
	"errors"
	"fmt"

	ocapi "github.com/FuturFusion/operations-center/shared/api"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Load gets a specific provider and initializes it with the provider configuration.
func Load(ctx context.Context, s *state.State, ignoreSignedJSON bool) (Provider, error) {
	var p Provider

	switch s.System.Provider.Config.Name {
	case "debug":
		// Setup the debug provider.
		p = &debug{
			state: s,
		}

	case "images":
		// Setup the images provider.
		p = &images{
			state:            s,
			ignoreSignedJSON: ignoreSignedJSON,
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

// Notify is a hook that is called whenever the current provider should be notified of an event.
func Notify(ctx context.Context, s *state.State, cause ocapi.ServerSelfUpdateCause) error {
	if s.System.Provider.Config.Name == "" {
		return nil
	}

	p, err := Load(ctx, s, false)
	if err != nil {
		return err
	}

	err = p.RefreshRegister(ctx, cause)
	if err != nil && !errors.Is(err, ErrRegistrationUnsupported) {
		return err
	}

	return nil
}
