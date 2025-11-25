package seed

import (
	"testing"

	"github.com/stretchr/testify/require"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

func TestGetApplications(t *testing.T) {
	t.Parallel()

	var apps apiseed.Applications

	err := parseFileContents("testdata.tar", "applications", &apps)

	require.NoError(t, err)

	require.Equal(t, "1.2.3", apps.Version)
	require.Len(t, apps.Applications, 2)
	require.Equal(t, "foo", apps.Applications[0].Name)
	require.Equal(t, "bar", apps.Applications[1].Name)
}
