package systemd

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
)

var networkdConfig1 = `
interfaces:
  - name: san1
    addresses:
      - 10.0.101.10/24
      - fd40:1234:1234:101::10/64
    hwaddr: AA:BB:CC:DD:EE:01
    roles:
      - storage

  - name: san2
    addresses:
      - 10.0.102.10/24
      - fd40:1234:1234:102::10/64
    hwaddr: AA:BB:CC:DD:EE:02
    roles:
      - storage

bonds:
  - name: management
    mode: 802.3ad
    mtu: 9000
    vlan: 100
    addresses:
      - 10.0.100.10/24
      - fd40:1234:1234:100::10/64
    routes:
      - to: 0.0.0.0/0
        via: 10.0.100.1
      - to: ::/0
        via: fd40:1234:1234:100::1
    members:
      - AA:BB:CC:DD:EE:03
      - AA:BB:CC:DD:EE:04
    roles:
      - management
      - instances

vlans:
  - name: uplink
    parent: management
    id: 1234
    mtu: 1500
    roles:
      - ovn-uplink
`

var networkdConfig2 = `
interfaces:
  - name: management
    addresses:
      - dhcp4
      - slaac
    routes:
      - to: 0.0.0.0/0
        via: dhcp4
      - to: ::/0
        via: slaac
    hwaddr: AA:BB:CC:DD:EE:01
    roles:
      - management
      - instances
`

func TestNetworkConfigMarshalling(t *testing.T) {
	t.Parallel()

	var cfg, cfgAgain seed.NetworkConfig

	// Test unmarshalling of the first test config.
	err := yaml.Unmarshal([]byte(networkdConfig1), &cfg)
	require.NoError(t, err)

	// Verify values were parsed correctly.
	require.Len(t, cfg.Interfaces, 2)
	require.Equal(t, "san1", cfg.Interfaces[0].Name)
	require.Len(t, cfg.Interfaces[0].Addresses, 2)
	require.Equal(t, "fd40:1234:1234:101::10/64", cfg.Interfaces[0].Addresses[1].String())
	require.Equal(t, "fd40:1234:1234:101::10", cfg.Interfaces[0].Addresses[1].Address.IP.String())
	require.Equal(t, "ffffffffffffffff0000000000000000", cfg.Interfaces[0].Addresses[1].Address.Mask.String())
	require.Len(t, cfg.Interfaces[1].Addresses, 2)
	require.Equal(t, "10.0.102.10/24", cfg.Interfaces[1].Addresses[0].String())
	require.Equal(t, "10.0.102.10", cfg.Interfaces[1].Addresses[0].Address.IP.String())
	require.Equal(t, "ffffff00", cfg.Interfaces[1].Addresses[0].Address.Mask.String())
	require.Equal(t, "aa:bb:cc:dd:ee:02", cfg.Interfaces[1].Hwaddr.String())
	require.Len(t, cfg.Interfaces[1].Roles, 1)
	require.Equal(t, "storage", cfg.Interfaces[1].Roles[0])
	require.Len(t, cfg.Bonds, 1)
	require.Equal(t, "management", cfg.Bonds[0].Name)
	require.Equal(t, 9000, cfg.Bonds[0].MTU)
	require.Len(t, cfg.Bonds[0].Routes, 2)
	require.Len(t, cfg.Bonds[0].Members, 2)
	require.Equal(t, "aa:bb:cc:dd:ee:03", cfg.Bonds[0].Members[0].String())
	require.Len(t, cfg.Vlans, 1)
	require.Equal(t, "uplink", cfg.Vlans[0].Name)
	require.Equal(t, 1234, cfg.Vlans[0].ID)
	require.Len(t, cfg.Vlans[0].Roles, 1)
	require.Equal(t, "ovn-uplink", cfg.Vlans[0].Roles[0])

	// Verify we can marshal and unmarshal the test config and don't loose any information.
	content, err := yaml.Marshal(&cfg)
	require.NoError(t, err)

	err = yaml.Unmarshal(content, &cfgAgain)
	require.NoError(t, err)
	require.Equal(t, cfg, cfgAgain)

	// Test unmarshalling of the second test config.
	err = yaml.Unmarshal([]byte(networkdConfig2), &cfg)
	require.NoError(t, err)

	// Verify values were parsed correctly.
	require.Len(t, cfg.Interfaces, 1)
	require.Equal(t, "management", cfg.Interfaces[0].Name)
	require.Len(t, cfg.Interfaces[0].Addresses, 2)
	require.Equal(t, "slaac", cfg.Interfaces[0].Addresses[1].String())
	require.Equal(t, "aa:bb:cc:dd:ee:01", cfg.Interfaces[0].Hwaddr.String())
	require.Len(t, cfg.Interfaces[0].Routes, 2)
	require.Equal(t, "0.0.0.0/0", cfg.Interfaces[0].Routes[0].To.String())
	require.Equal(t, "dhcp4", cfg.Interfaces[0].Routes[0].Via.String())

	// Verify we can marshal and unmarshal the test config and don't loose any information.
	content, err = yaml.Marshal(&cfg)
	require.NoError(t, err)

	err = yaml.Unmarshal(content, &cfgAgain)
	require.NoError(t, err)
	require.Equal(t, cfg, cfgAgain)
}

func TestLinkFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg seed.NetworkConfig

	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "00-enaabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nMACAddress=aa:bb:cc:dd:ee:01\n\n[Link]\nNamePolicy=\nName=enaabbccddee01\n", cfgs[0].Contents)
	require.Equal(t, "00-enaabbccddee02.link", cfgs[1].Name)
	require.Equal(t, "[Match]\nMACAddress=aa:bb:cc:dd:ee:02\n\n[Link]\nNamePolicy=\nName=enaabbccddee02\n", cfgs[1].Contents)

	networkCfg = seed.NetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-enaabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nMACAddress=aa:bb:cc:dd:ee:01\n\n[Link]\nNamePolicy=\nName=enaabbccddee01\n", cfgs[0].Contents)
}

func TestNetdevFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg seed.NetworkConfig

	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 4)
	require.Equal(t, "00-san1.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=san1\nKind=bridge\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
	require.Equal(t, "00-san2.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=san2\nKind=bridge\n\n[Bridge]\nVLANFiltering=true\n", cfgs[1].Contents)
	require.Equal(t, "00-bnmanagement.netdev", cfgs[2].Name)
	require.Equal(t, "[NetDev]\nName=bnmanagement\nKind=bond\nMTUBytes=9000\n\n[Bond]\nMode=802.3ad\n", cfgs[2].Contents)
	require.Equal(t, "00-vluplink.netdev", cfgs[3].Name)
	require.Equal(t, "[NetDev]\nName=vluplink\nKind=vlan\nMTUBytes=1500\n\n[Bridge]\nId=1234\n", cfgs[3].Contents)

	networkCfg = seed.NetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-management.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=management\nKind=bridge\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
}

func TestNetworkFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg seed.NetworkConfig

	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 7)
	require.Equal(t, "00-san1.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=san1\n\n[Network]\nAddress=10.0.101.10/24\nAddress=fd40:1234:1234:101::10/64\n", cfgs[0].Contents)
	require.Equal(t, "00-enaabbccddee01.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee01\n\n[Network]\nBridge=san1\n", cfgs[1].Contents)
	require.Equal(t, "00-san2.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=san2\n\n[Network]\nAddress=10.0.102.10/24\nAddress=fd40:1234:1234:102::10/64\n", cfgs[2].Contents)
	require.Equal(t, "00-enaabbccddee02.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee02\n\n[Network]\nBridge=san2\n", cfgs[3].Contents)
	require.Equal(t, "00-bnmanagement.network", cfgs[4].Name)
	require.Equal(t, "[Match]\nName=bnmanagement\n\n[Network]\nAddress=10.0.100.10/24\nAddress=fd40:1234:1234:100::10/64\n\n[Route]\nGateway=10.0.100.1/32\nDestination=0.0.0.0/0\nGateway=fd40:1234:1234:100::1/128\nDestination=::/0\n\n[BridgeVLAN]\nVLAN=100\n", cfgs[4].Contents)
	require.Equal(t, "00-bnmanagement-dev0.network", cfgs[5].Name)
	require.Equal(t, "[Match]\nMACAddress=aa:bb:cc:dd:ee:03\n\n[Network]\nBond=bnmanagement\n", cfgs[5].Contents)
	require.Equal(t, "00-bnmanagement-dev1.network", cfgs[6].Name)
	require.Equal(t, "[Match]\nMACAddress=aa:bb:cc:dd:ee:04\n\n[Network]\nBond=bnmanagement\n", cfgs[6].Contents)

	networkCfg = seed.NetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "00-management.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=management\n\n[Network]\nIPv6AcceptRA=true\nDHCP=ipv4\n\n[Route]\nGateway=_dhcp4\nDestination=0.0.0.0/0\nGateway=_ipv6ra\nDestination=::/0\n", cfgs[0].Contents)
	require.Equal(t, "00-enaabbccddee01.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee01\n\n[Network]\nBridge=management\n", cfgs[1].Contents)
}
