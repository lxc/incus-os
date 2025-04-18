package systemd

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/lxc/incus-os/incus-osd/api"
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

var networkdConfig3 = `
version: 1.2.3
dns:
  hostname: host
  domain: example.org
  search_domains:
    - example.org
  nameservers:
    - ns1.example.org
    - ns2.example.org
ntp:
  timeservers:
    - pool.ntp.example.org
    - 10.10.10.10
proxy:
  https_proxy: https://proxy.example.org
interfaces:
  - name: eth0
    addresses:
      - dhcp4
    hwaddr: FF:EE:DD:CC:BB:AA
`

func TestNetworkConfigMarshalling(t *testing.T) {
	t.Parallel()

	{
		var cfg, cfgAgain api.SystemNetwork

		// Test unmarshalling of the first test config.
		err := yaml.Unmarshal([]byte(networkdConfig1), &cfg)
		require.NoError(t, err)

		// Verify values were parsed correctly.
		require.Len(t, cfg.Interfaces, 2)
		require.Equal(t, "san1", cfg.Interfaces[0].Name)
		require.Len(t, cfg.Interfaces[0].Addresses, 2)
		require.Equal(t, "fd40:1234:1234:101::10/64", cfg.Interfaces[0].Addresses[1])
		require.Len(t, cfg.Interfaces[1].Addresses, 2)
		require.Equal(t, "10.0.102.10/24", cfg.Interfaces[1].Addresses[0])
		require.Equal(t, "AA:BB:CC:DD:EE:02", cfg.Interfaces[1].Hwaddr)
		require.Len(t, cfg.Interfaces[1].Roles, 1)
		require.Equal(t, "storage", cfg.Interfaces[1].Roles[0])
		require.Len(t, cfg.Bonds, 1)
		require.Equal(t, "management", cfg.Bonds[0].Name)
		require.Equal(t, 9000, cfg.Bonds[0].MTU)
		require.Len(t, cfg.Bonds[0].Routes, 2)
		require.Len(t, cfg.Bonds[0].Members, 2)
		require.Equal(t, "AA:BB:CC:DD:EE:03", cfg.Bonds[0].Members[0])
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
	}

	{
		var cfg, cfgAgain api.SystemNetwork

		// Test unmarshalling of the second test config.
		err := yaml.Unmarshal([]byte(networkdConfig2), &cfg)
		require.NoError(t, err)

		// Verify values were parsed correctly.
		require.Len(t, cfg.Interfaces, 1)
		require.Equal(t, "management", cfg.Interfaces[0].Name)
		require.Len(t, cfg.Interfaces[0].Addresses, 2)
		require.Equal(t, "slaac", cfg.Interfaces[0].Addresses[1])
		require.Equal(t, "AA:BB:CC:DD:EE:01", cfg.Interfaces[0].Hwaddr)
		require.Len(t, cfg.Interfaces[0].Routes, 2)
		require.Equal(t, "0.0.0.0/0", cfg.Interfaces[0].Routes[0].To)
		require.Equal(t, "dhcp4", cfg.Interfaces[0].Routes[0].Via)

		// Verify we can marshal and unmarshal the test config and don't loose any information.
		content, err := yaml.Marshal(&cfg)
		require.NoError(t, err)

		err = yaml.Unmarshal(content, &cfgAgain)
		require.NoError(t, err)
		require.Equal(t, cfg, cfgAgain)
	}

	{
		var cfg, cfgAgain api.SystemNetwork

		// Test unmarshalling of the third test config.
		err := yaml.Unmarshal([]byte(networkdConfig3), &cfg)
		require.NoError(t, err)

		// Verify values were parsed correctly.
		require.Equal(t, "1.2.3", cfg.Version)
		require.Equal(t, "host", cfg.DNS.Hostname)
		require.Equal(t, "example.org", cfg.DNS.Domain)
		require.Len(t, cfg.DNS.SearchDomains, 1)
		require.Equal(t, "example.org", cfg.DNS.SearchDomains[0])
		require.Len(t, cfg.DNS.Nameservers, 2)
		require.Equal(t, "ns1.example.org", cfg.DNS.Nameservers[0])
		require.Equal(t, "ns2.example.org", cfg.DNS.Nameservers[1])
		require.Len(t, cfg.NTP.Timeservers, 2)
		require.Equal(t, "pool.ntp.example.org", cfg.NTP.Timeservers[0])
		require.Equal(t, "10.10.10.10", cfg.NTP.Timeservers[1])
		require.Equal(t, "https://proxy.example.org", cfg.Proxy.HTTPSProxy)

		// Verify we can marshal and unmarshal the test config and don't loose any information.
		content, err := yaml.Marshal(&cfg)
		require.NoError(t, err)

		err = yaml.Unmarshal(content, &cfgAgain)
		require.NoError(t, err)
		require.Equal(t, cfg, cfgAgain)
	}
}

func TestLinkFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg api.SystemNetwork

	// Test first config .link file generation.
	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "00-enaabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nMACAddress=AA:BB:CC:DD:EE:01\n\n[Link]\nNamePolicy=\nName=enaabbccddee01\n", cfgs[0].Contents)
	require.Equal(t, "00-enaabbccddee02.link", cfgs[1].Name)
	require.Equal(t, "[Match]\nMACAddress=AA:BB:CC:DD:EE:02\n\n[Link]\nNamePolicy=\nName=enaabbccddee02\n", cfgs[1].Contents)

	// Test second config .link file generation.
	networkCfg = api.SystemNetwork{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-enaabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nMACAddress=AA:BB:CC:DD:EE:01\n\n[Link]\nNamePolicy=\nName=enaabbccddee01\n", cfgs[0].Contents)

	// Test third config .link file generation.
	networkCfg = api.SystemNetwork{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-enffeeddccbbaa.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nMACAddress=FF:EE:DD:CC:BB:AA\n\n[Link]\nNamePolicy=\nName=enffeeddccbbaa\n", cfgs[0].Contents)
}

func TestNetdevFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg api.SystemNetwork

	// Test first config .netdev file generation.
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

	// Test second config .netdev file generation.
	networkCfg = api.SystemNetwork{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-management.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=management\nKind=bridge\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)

	// Test third config .netdev file generation.
	networkCfg = api.SystemNetwork{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-eth0.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=eth0\nKind=bridge\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
}

func TestNetworkFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg api.SystemNetwork

	// Test first config .network file generation.
	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 7)
	require.Equal(t, "00-san1.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=san1\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLLDP=false\nEmitLLDP=false\nLinkLocalAddressing=ipv6\nAddress=10.0.101.10/24\nAddress=fd40:1234:1234:101::10/64\n", cfgs[0].Contents)
	require.Equal(t, "00-enaabbccddee01.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee01\n\n[Network]\nBridge=san1\n", cfgs[1].Contents)
	require.Equal(t, "00-san2.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=san2\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLLDP=false\nEmitLLDP=false\nLinkLocalAddressing=ipv6\nAddress=10.0.102.10/24\nAddress=fd40:1234:1234:102::10/64\n", cfgs[2].Contents)
	require.Equal(t, "00-enaabbccddee02.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee02\n\n[Network]\nBridge=san2\n", cfgs[3].Contents)
	require.Equal(t, "00-bnmanagement.network", cfgs[4].Name)
	require.Equal(t, "[Match]\nName=bnmanagement\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLLDP=false\nEmitLLDP=false\nLinkLocalAddressing=ipv6\nAddress=10.0.100.10/24\nAddress=fd40:1234:1234:100::10/64\n\n[Route]\nGateway=10.0.100.1\nDestination=0.0.0.0/0\nGateway=fd40:1234:1234:100::1\nDestination=::/0\n\n[BridgeVLAN]\nVLAN=100\n", cfgs[4].Contents)
	require.Equal(t, "00-bnmanagement-dev0.network", cfgs[5].Name)
	require.Equal(t, "[Match]\nMACAddress=AA:BB:CC:DD:EE:03\n\n[Network]\nBond=bnmanagement\n", cfgs[5].Contents)
	require.Equal(t, "00-bnmanagement-dev1.network", cfgs[6].Name)
	require.Equal(t, "[Match]\nMACAddress=AA:BB:CC:DD:EE:04\n\n[Network]\nBond=bnmanagement\n", cfgs[6].Contents)

	// Test second config .network file generation.
	networkCfg = api.SystemNetwork{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "00-management.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=management\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLLDP=false\nEmitLLDP=false\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=true\nDHCP=ipv4\n\n[Route]\nGateway=_dhcp4\nDestination=0.0.0.0/0\nGateway=_ipv6ra\nDestination=::/0\n", cfgs[0].Contents)
	require.Equal(t, "00-enaabbccddee01.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee01\n\n[Network]\nBridge=management\n", cfgs[1].Contents)

	// Test third config .network file generation.
	networkCfg = api.SystemNetwork{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "00-eth0.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=eth0\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLLDP=false\nEmitLLDP=false\nLinkLocalAddressing=ipv6\nDomains=example.org\nDNS=ns1.example.org\nDNS=ns2.example.org\nNTP=pool.ntp.example.org\nNTP=10.10.10.10\nDHCP=ipv4\n", cfgs[0].Contents)
	require.Equal(t, "00-enffeeddccbbaa.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=enffeeddccbbaa\n\n[Network]\nBridge=eth0\n", cfgs[1].Contents)
}
