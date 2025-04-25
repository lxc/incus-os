package seed

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/api"
)

func TestGetFileContents(t *testing.T) {
	t.Parallel()

	var config api.SystemNetworkConfig
	err := parseFileContents("testdata.tar", "network", &config)
	require.NoError(t, err)
}
