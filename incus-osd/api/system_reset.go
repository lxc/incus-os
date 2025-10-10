package api

import (
	"encoding/json"
)

// SystemReset defines a struct that takes an optional map of seed data to set as part of the factory reset.
type SystemReset struct {
	AllowTPMResetFailure bool                       `json:"allow_tpm_reset_failure" yaml:"allow_tpm_reset_failure"`
	Seeds                map[string]json.RawMessage `json:"seeds"                   yaml:"seeds"`
	WipeExistingSeeds    bool                       `json:"wipe_existing_seeds"     yaml:"wipe_existing_seeds"`
}
