package state_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

var goldJSON = `{"secure_boot":{"version":"","fully_applied":false},"applications":{"incus":{"initialized":true,"version":"202506241635"}},"os":{"name":"IncusOS","running_release":"202506241635","next_release":"202506241635"},"services":{"iscsi":{"state":{"initiator_name":""},"config":{"enabled":false,"targets":null}},"lvm":{"state":{},"config":{"enabled":false,"system_id":0}},"nvme":{"state":{"host_id":"","host_nqn":""},"config":{"enabled":false,"targets":null}},"ovn":{"state":{},"config":{"enabled":false,"ic_chassis":false,"database":"","tls_client_certificate":"","tls_client_key":"","tls_ca_certificate":"","tunnel_address":"","tunnel_protocol":""}},"usbip":{"state":{},"config":{"targets":null}}},"system":{"security":{"config":{"encryption_recovery_keys":["ebbbibiu-ltgjfuhk-gvutdrvu-hijhvfje-gvlrgrfv-ndekdtdh-ghteuklj-ldedfifb"]},"state":{"encrypted_volumes":[],"encryption_recovery_keys_retrieved":true,"secure_boot_certificates":[],"secure_boot_enabled":false,"tpm_status":""}},"network":{"config":{"interfaces":[{"name":"enp5s0","addresses":["dhcp4","slaac"],"hwaddr":"10:66:6a:7c:8c:b0","lldp":false}]},"state":{"interfaces":{"enp5s0":{"type":"interface","hwaddr":"10:66:6a:7c:8c:b0","addresses":["10.234.136.156"],"routes":[{"to":"default","via":"10.234.136.1"}],"mtu":1500,"speed":"10Gbps","state":"routable","stats":{"rx_bytes":944,"tx_bytes":751,"rx_errors":0,"tx_errors":0}}}}},"provider":{"config":{"name":"local","config":null},"state":{"registered":false}}}}`

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
`

// Test basic json decoding/encoding of state.
func TestJsonEncoding(t *testing.T) {
	t.Parallel()

	var s state.State

	err := json.Unmarshal([]byte(goldJSON), &s)
	require.NoError(t, err)

	content, err := json.Marshal(s)
	require.NoError(t, err)

	require.JSONEq(t, goldJSON, string(content))
}

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
}

// Test that we can correctly read in the json format and produce the expected custom encoding.
func TestEncodingSwitch(t *testing.T) {
	t.Parallel()

	var js state.State

	err := json.Unmarshal([]byte(goldJSON), &js)
	require.NoError(t, err)

	content, err := state.Encode(&js)
	require.NoError(t, err)

	var cs1, cs2 state.State

	err = state.Decode(content, nil, &cs1)
	require.NoError(t, err)

	err = state.Decode([]byte(goldEncodingV1), nil, &cs2)
	require.NoError(t, err)

	require.Equal(t, cs1, cs2)
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
