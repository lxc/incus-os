package state

import (
	"context"
	"encoding/json"
	"os"
	"strings"
)

// LoadOrCreate parses the on-disk state file and returns a State struct.
// If no file exists, a new empty one is created.
func LoadOrCreate(ctx context.Context, path string) (*State, error) {
	s := State{
		path: path,

		Applications: map[string]Application{},
	}

	body, err := os.ReadFile(s.path)
	if err == nil {
		err = Decode(body, nil, &s)

		return &s, err
	}

	if os.IsNotExist(err) {
		// Check if a legacy json state file exists.
		_, err := os.Stat(strings.Replace(s.path, ".txt", ".json", 1))
		if err == nil {
			err := convertLegacyJSON(ctx, strings.Replace(s.path, ".txt", ".json", 1), &s)
			if err != nil {
				return nil, err
			}

			return &s, nil
		}

		// State file doesn't exist, create it and return it.
		err = s.Save(ctx)
		if err != nil {
			return nil, err
		}

		return &s, nil
	}

	return nil, err
}

// convertLegacyJSON reads a legacy json state file, saves using the new format and removes the old json file.
func convertLegacyJSON(ctx context.Context, path string, s *State) error {
	// #nosec G304
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, s)
	if err != nil {
		return err
	}

	err = s.Save(ctx)
	if err != nil {
		return err
	}

	return os.Remove(path)
}

// Save writes out the current state struct into its on-disk storage.
func (s *State) Save(_ context.Context) error {
	body, err := Encode(s)
	if err != nil {
		return err
	}

	err = os.WriteFile(s.path, body, 0o600)
	if err != nil {
		return err
	}

	return nil
}
