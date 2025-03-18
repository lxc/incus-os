package seed

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetFileContents(t *testing.T) {
	t.Parallel()

	var config NetworkConfig
	err := parseFileContents("testdata.tar", "network", &config)
	require.NoError(t, err)
}
