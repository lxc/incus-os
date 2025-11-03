package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Make sure that we have correctly bumped the schema version.
func TestSchemaVersion(t *testing.T) {
	t.Parallel()

	require.Equal(t, len(upgrades), currentStateVersion)
}
