package api

import (
	"encoding/json"
)

// SystemReset defines a struct that takes an optional map of seed data to set as part of the factory reset.
type SystemReset struct {
	Seed map[string]json.RawMessage `json:"seed" yaml:"seed"`
}
