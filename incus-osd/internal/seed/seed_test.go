package seed

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/api"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

func TestGoodSeed(t *testing.T) {
	t.Parallel()

	var config apiseed.Applications

	err := parseFileContents("testdata.tar", "applications", &config)

	require.NoError(t, err)
	require.Len(t, config.Applications, 2)
	require.Equal(t, "foo", config.Applications[0].Name)
}

func TestSeedLeadingDotSlash(t *testing.T) {
	t.Parallel()

	var config apiseed.Kernel

	err := parseFileContents("testdata.tar", "kernel", &config)

	require.NoError(t, err)
	require.Len(t, config.Console, 1)
	require.Equal(t, "/dev/ttyS0", config.Console[0].Device)
}

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
