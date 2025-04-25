package state

import (
	"context"
	"encoding/json"
	"os"

	"github.com/lxc/incus-os/incus-osd/api"
)

// LoadOrCreate parses the on-disk state file and returns a State struct.
// If no file exists, a new empty one is created.
func LoadOrCreate(ctx context.Context, path string) (*State, error) {
	s := State{
		path: path,

		Applications: map[string]Application{},
	}

	s.System.Encryption = &api.SystemEncryption{}

	body, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			// State file doesn't exist, create it and return it.
			err = s.Save(ctx)
			if err != nil {
				return nil, err
			}

			return &s, nil
		}

		return nil, err
	}

	err = json.Unmarshal(body, &s)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// Save writes out the current state struct into its on-disk storage.
func (s *State) Save(_ context.Context) error {
	body, err := json.Marshal(s)
	if err != nil {
		return err
	}

	err = os.WriteFile(s.path, body, 0o600)
	if err != nil {
		return err
	}

	return nil
}
