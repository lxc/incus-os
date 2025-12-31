package seed

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/api"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

func TestBadJSONFields(t *testing.T) {
	t.Parallel()

	var config api.SystemNetworkConfig

	err := parseFileContents("testdata.tar", "network", &config)

	require.Error(t, err, "json: unknown field \"config\"")
}

func TestBadYAMLFields(t *testing.T) {
	t.Parallel()

	var config apiseed.Install

	err := parseFileContents("testdata.tar", "install", &config)

	require.Error(t, err, "line 3: field disable_everything not found in type seed.InstallSecurity")
}
