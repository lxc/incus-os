package state_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

var goldEncodingV0 = `#Version: 0
Applications[incus].Initialized: true
Applications[incus].Version: 202506241635
OS.Name: IncusOS
OS.RunningRelease: 202506241635
OS.NextRelease: 202506241635
System.Encryption.Config.RecoveryKeys[0]: ebbbibiu-ltgjfuhk-gvutdrvu-hijhvfje-gvlrgrfv-ndekdtdh-ghteuklj-ldedfifb
System.Encryption.State.RecoveryKeysRetrieved: true
System.Network.Config.Interfaces[0].Name: enp5s0
System.Network.Config.Interfaces[0].Addresses[0]: dhcp4
System.Network.Config.Interfaces[0].Addresses[1]: slaac
System.Network.Config.Interfaces[0].Hwaddr: 10:66:6a:7c:8c:b0
System.Provider.Config.Name: local
System.Provider.Config.Config[multiline_value]: first\nsecond\nthird
`

var goldEncodingV1 = `#Version: 1
Applications[incus].Initialized: true
Applications[incus].Version: 202506241635
OS.Name: IncusOS
OS.RunningRelease: 202506241635
OS.NextRelease: 202506241635
System.Security.Config.EncryptionRecoveryKeys[0]: ebbbibiu-ltgjfuhk-gvutdrvu-hijhvfje-gvlrgrfv-ndekdtdh-ghteuklj-ldedfifb
System.Security.State.EncryptionRecoveryKeysRetrieved: true
System.Network.Config.Interfaces[0].Name: enp5s0
System.Network.Config.Interfaces[0].Addresses[0]: dhcp4
System.Network.Config.Interfaces[0].Addresses[1]: slaac
System.Network.Config.Interfaces[0].Hwaddr: 10:66:6a:7c:8c:b0
System.Provider.Config.Name: local
System.Provider.Config.Config[multiline_value]: first\nsecond\nthird
`

// Test basic custom decoding/encoding of state.
func TestCustomEncoding(t *testing.T) {
	t.Parallel()

	var s state.State

	err := state.Decode([]byte(goldEncodingV0), nil, &s)
	require.NoError(t, err)

	content, err := state.Encode(&s)
	require.NoError(t, err)

	require.Equal(t, goldEncodingV1, string(content))
	require.Equal(t, 1, s.StateVersion)

	require.Equal(t, 2, strings.Count(s.System.Provider.Config.Config["multiline_value"], "\n"))
}

// Test simple upgrade functions when reading in state.
func TestUpgradeFuncs(t *testing.T) {
	t.Parallel()

	funcs := state.UpgradeFuncs{
		nil,
		func(lines []string) ([]string, error) {
			for i, line := range lines {
				if strings.HasPrefix(line, "OS.Name") {
					lines[i] = "OS.Name: My Test OS"
				}
			}

			return lines, nil
		},
		func(lines []string) ([]string, error) {
			for i, line := range lines {
				if strings.HasPrefix(line, "System.Network.Config.Interfaces[0].Addresses[1]") {
					lines[i] = "System.Network.Config.Interfaces[0].Addresses[1]: dhcp6"
				}
			}

			return lines, nil
		},
	}

	var s state.State

	err := state.Decode([]byte(goldEncodingV1), funcs, &s)
	require.NoError(t, err)

	require.Equal(t, 3, s.StateVersion)
	require.Equal(t, "My Test OS", s.OS.Name)
	require.Equal(t, "dhcp4", s.System.Network.Config.Interfaces[0].Addresses[0])
	require.Equal(t, "dhcp6", s.System.Network.Config.Interfaces[0].Addresses[1])
}
