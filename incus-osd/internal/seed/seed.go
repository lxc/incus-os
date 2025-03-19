package seed

import (
	"errors"
)

// IsMissing checks whether the provided error is an expected error for missing seed data.
func IsMissing(e error) bool {
	for _, entry := range []error{ErrNoSeedPartition, ErrNoSeedData, ErrNoSeedSection} {
		if errors.Is(e, entry) {
			return true
		}
	}

	return false
}
