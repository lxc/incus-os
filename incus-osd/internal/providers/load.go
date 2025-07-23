package providers

import (
	"context"
	"fmt"
	"slices"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Load gets a specific provider and initializes it with the provider configuration.
func Load(ctx context.Context, s *state.State, name string, config map[string]string) (Provider, error) {
	// NOTE: Migration logic, remove after a few releases.
	if name == "github" {
		if s.System.Provider.Config.Name == "github" {
			s.System.Provider.Config.Name = "images"
		}

		name = "images"
	}

	if !slices.Contains([]string{"images", "local", "operations-center"}, name) {
		return nil, fmt.Errorf("unknown provider %q", name)
	}

	var p Provider

	switch name {
	case "images":
		// Setup the images provider.
		p = &images{
			config: config,
			state:  s,
		}

	case "local":
		// Setup the local provider.
		p = &local{
			config: config,
			state:  s,
		}

	case "operations-center":
		// Setup the Operations Center provider.
		p = &operationsCenter{
			config: config,
			state:  s,
		}

	default:
	}

	err := p.load(ctx)
	if err != nil {
		return nil, err
	}

	return p, nil
}
