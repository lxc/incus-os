package providers

import (
	"context"
	"fmt"
	"slices"
)

// Load gets a specific provider and initializes it with the provider configuration.
func Load(ctx context.Context, name string, config map[string]string) (Provider, error) {
	if !slices.Contains([]string{"github", "local"}, name) {
		return nil, fmt.Errorf("unknown provider %q", name)
	}

	var p Provider

	switch name {
	case "github":
		// Setup the Github provider.
		p = &github{
			config: config,
		}

	case "local":
		// Setup the Github provider.
		p = &local{
			config: config,
		}
	}

	err := p.load(ctx)
	if err != nil {
		return nil, err
	}

	return p, nil
}
