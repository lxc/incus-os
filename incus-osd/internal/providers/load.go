package providers

import (
	"context"
	"fmt"
)

// Load gets a specific provider and initializes it with the provider configuration.
func Load(ctx context.Context, name string, config map[string]string) (Provider, error) {
	if name != "github" {
		return nil, fmt.Errorf("unknown provider %q", name)
	}

	// Setup the Github provider.
	provider := github{
		config: config,
	}

	err := provider.load(ctx)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}
