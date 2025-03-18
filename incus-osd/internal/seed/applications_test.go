package seed_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
)

func TestGetApplications(t *testing.T) {
	t.Parallel()

	apps, err := seed.GetApplications(context.TODO(), "testdata.tar")

	require.NoError(t, err)

	require.Equal(t, "1.2.3", apps.Version)
	require.Len(t, apps.Applications, 2)
	require.Equal(t, "foo", apps.Applications[0].Name)
	require.Equal(t, "bar", apps.Applications[1].Name)
}
