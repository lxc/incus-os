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
    required_for_online: both
    hwaddr: AA:BB:CC:DD:EE:01
    roles:
      - storage

  - name: san2
    addresses:
      - 10.0.102.10/24
      - fd40:1234:1234:102::10/64
    required_for_online: both
    hwaddr: AA:BB:CC:DD:EE:02
    vlan_tags:
      - 10
    roles:
      - storage

bonds:
  - name: management
    mode: 802.3ad
    mtu: 9000
    vlan: 1234
    vlan_tags:
      - 100
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
    addresses:
      - dhcp4
    required_for_online: ipv4
    routes:
      - to: 0.0.0.0/0
        via: dhcp4
    roles:
      - ovn-uplink
`

var networkdConfig2 = `
interfaces:
  - name: management
    mtu: 9000
    addresses:
      - dhcp4
      - slaac
    required_for_online: ipv6
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
    required_for_online: no
    hwaddr: FF:EE:DD:CC:BB:AA
`

var networkdConfig4 = `
bonds:
 - name: "uplink"
   mode: "802.3ad"
   hwaddr: "aa:bb:cc:dd:ee:e1"
   lldp: true
   mtu: 9000
   vlan: 10
   members:
    - "aa:bb:cc:dd:ee:e1"
    - "aa:bb:cc:dd:ee:e2"
   roles:
    - "instances"

vlans:
 - name: "management"
   id: 10
   parent: "uplink"
   mtu: 1500
   addresses:
    - "dhcp4"
    - "slaac"
   required_for_online: both
   roles:
    - "management"
`

func TestNetworkConfigMarshalling(t *testing.T) {
	t.Parallel()

	{
		var cfg, cfgAgain api.SystemNetworkConfig

		// Test unmarshalling of the first test config.
		err := yaml.Unmarshal([]byte(networkdConfig1), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, true)
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
		require.Empty(t, cfg.Bonds[0].Hwaddr)
		require.Len(t, cfg.Bonds[0].Routes, 2)
		require.Len(t, cfg.Bonds[0].Members, 2)
		require.Equal(t, "AA:BB:CC:DD:EE:03", cfg.Bonds[0].Members[0])
		require.Len(t, cfg.VLANs, 1)
		require.Equal(t, "uplink", cfg.VLANs[0].Name)
		require.Equal(t, 1234, cfg.VLANs[0].ID)
		require.Len(t, cfg.VLANs[0].Addresses, 1)
		require.Equal(t, "dhcp4", cfg.VLANs[0].Addresses[0])
		require.Len(t, cfg.VLANs[0].Routes, 1)
		require.Equal(t, "0.0.0.0/0", cfg.VLANs[0].Routes[0].To)
		require.Equal(t, "dhcp4", cfg.VLANs[0].Routes[0].Via)
		require.Len(t, cfg.VLANs[0].Roles, 1)
		require.Equal(t, "ovn-uplink", cfg.VLANs[0].Roles[0])

		// Verify we can marshal and unmarshal the test config and don't loose any information.
		content, err := yaml.Marshal(&cfg)
		require.NoError(t, err)

		err = yaml.Unmarshal(content, &cfgAgain)
		require.NoError(t, err)
		require.Equal(t, cfg, cfgAgain)
	}

	{
		var cfg, cfgAgain api.SystemNetworkConfig

		// Test unmarshalling of the second test config.
		err := yaml.Unmarshal([]byte(networkdConfig2), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, true)
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
		var cfg, cfgAgain api.SystemNetworkConfig

		// Test unmarshalling of the third test config.
		err := yaml.Unmarshal([]byte(networkdConfig3), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, true)
		require.NoError(t, err)

		// Verify values were parsed correctly.
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

	{
		var cfg, cfgAgain api.SystemNetworkConfig

		// Test unmarshalling of the fourth test config.
		err := yaml.Unmarshal([]byte(networkdConfig4), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, true)
		require.NoError(t, err)

		// Verify values were parsed correctly.
		require.Empty(t, cfg.Interfaces)
		require.Len(t, cfg.Bonds, 1)
		require.Equal(t, "uplink", cfg.Bonds[0].Name)
		require.Equal(t, "802.3ad", cfg.Bonds[0].Mode)
		require.Equal(t, "aa:bb:cc:dd:ee:e1", cfg.Bonds[0].Hwaddr)
		require.True(t, cfg.Bonds[0].LLDP)
		require.Equal(t, 9000, cfg.Bonds[0].MTU)
		require.Len(t, cfg.Bonds[0].Members, 2)
		require.Equal(t, "aa:bb:cc:dd:ee:e1", cfg.Bonds[0].Members[0])
		require.Equal(t, "aa:bb:cc:dd:ee:e2", cfg.Bonds[0].Members[1])
		require.Len(t, cfg.Bonds[0].Roles, 1)
		require.Equal(t, "instances", cfg.Bonds[0].Roles[0])
		require.Len(t, cfg.VLANs, 1)
		require.Equal(t, "management", cfg.VLANs[0].Name)
		require.Equal(t, 10, cfg.VLANs[0].ID)
		require.Equal(t, "uplink", cfg.VLANs[0].Parent)
		require.Equal(t, 1500, cfg.VLANs[0].MTU)
		require.Len(t, cfg.VLANs[0].Addresses, 2)
		require.Equal(t, "dhcp4", cfg.VLANs[0].Addresses[0])
		require.Equal(t, "slaac", cfg.VLANs[0].Addresses[1])
		require.Len(t, cfg.VLANs[0].Roles, 1)
		require.Equal(t, "management", cfg.VLANs[0].Roles[0])

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

	var networkCfg api.SystemNetworkConfig

	// Test first config .link file generation.
	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 4)
	require.Equal(t, "00-enaabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:01\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=enaabbccddee01\n", cfgs[0].Contents)
	require.Equal(t, "00-enaabbccddee02.link", cfgs[1].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:02\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=enaabbccddee02\n", cfgs[1].Contents)
	require.Equal(t, "01-enaabbccddee03.link", cfgs[2].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:03\n\n[Link]\nNamePolicy=\nName=enaabbccddee03\n", cfgs[2].Contents)
	require.Equal(t, "01-enaabbccddee04.link", cfgs[3].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:04\n\n[Link]\nNamePolicy=\nName=enaabbccddee04\n", cfgs[3].Contents)

	// Test second config .link file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-enaabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:01\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=enaabbccddee01\n", cfgs[0].Contents)

	// Test third config .link file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-enffeeddccbbaa.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=FF:EE:DD:CC:BB:AA\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=enffeeddccbbaa\n", cfgs[0].Contents)

	// Test fourth config .link file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig4), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "01-enaabbccddeee1.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=aa:bb:cc:dd:ee:e1\n\n[Link]\nNamePolicy=\nName=enaabbccddeee1\n", cfgs[0].Contents)
	require.Equal(t, "01-enaabbccddeee2.link", cfgs[1].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=aa:bb:cc:dd:ee:e2\n\n[Link]\nNamePolicy=\nName=enaabbccddeee2\n", cfgs[1].Contents)
}

func TestNetdevFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg api.SystemNetworkConfig

	// Test first config .netdev file generation.
	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 8)
	require.Equal(t, "10-brsan1.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=brsan1\nKind=bridge\n\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
	require.Equal(t, "10-san1.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=san1\nKind=veth\nMACAddress=AA:BB:CC:DD:EE:01\n\n\n[Peer]\nName=vtsan1\n", cfgs[1].Contents)
	require.Equal(t, "10-brsan2.netdev", cfgs[2].Name)
	require.Equal(t, "[NetDev]\nName=brsan2\nKind=bridge\n\n\n[Bridge]\nVLANFiltering=true\n", cfgs[2].Contents)
	require.Equal(t, "10-san2.netdev", cfgs[3].Name)
	require.Equal(t, "[NetDev]\nName=san2\nKind=veth\nMACAddress=AA:BB:CC:DD:EE:02\n\n\n[Peer]\nName=vtsan2\n", cfgs[3].Contents)
	require.Equal(t, "11-bnmanagement.netdev", cfgs[4].Name)
	require.Equal(t, "[NetDev]\nName=bnmanagement\nKind=bond\nMTUBytes=9000\n\n[Bond]\nMode=802.3ad\n", cfgs[4].Contents)
	require.Equal(t, "11-brmanagement.netdev", cfgs[5].Name)
	require.Equal(t, "[NetDev]\nName=brmanagement\nKind=bridge\nMTUBytes=9000\n\n[Bridge]\nVLANFiltering=true\n", cfgs[5].Contents)
	require.Equal(t, "11-management.netdev", cfgs[6].Name)
	require.Equal(t, "[NetDev]\nName=management\nKind=veth\nMACAddress=AA:BB:CC:DD:EE:03\nMTUBytes=9000\n\n[Peer]\nName=vtmanagement\n", cfgs[6].Contents)
	require.Equal(t, "12-uplink.netdev", cfgs[7].Name)
	require.Equal(t, "[NetDev]\nName=uplink\nKind=vlan\nMTUBytes=1500\n\n[VLAN]\nId=1234\n", cfgs[7].Contents)

	// Test second config .netdev file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "10-brmanagement.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=brmanagement\nKind=bridge\nMTUBytes=9000\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
	require.Equal(t, "10-management.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=management\nKind=veth\nMACAddress=AA:BB:CC:DD:EE:01\nMTUBytes=9000\n\n[Peer]\nName=vtmanagement\n", cfgs[1].Contents)

	// Test third config .netdev file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "10-breth0.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=breth0\nKind=bridge\n\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
	require.Equal(t, "10-eth0.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=eth0\nKind=veth\nMACAddress=FF:EE:DD:CC:BB:AA\n\n\n[Peer]\nName=vteth0\n", cfgs[1].Contents)

	// Test fourth config .netdev file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig4), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 4)
	require.Equal(t, "11-bnuplink.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=bnuplink\nKind=bond\nMTUBytes=9000\n\n[Bond]\nMode=802.3ad\n", cfgs[0].Contents)
	require.Equal(t, "11-bruplink.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=bruplink\nKind=bridge\nMTUBytes=9000\n\n[Bridge]\nVLANFiltering=true\n", cfgs[1].Contents)
	require.Equal(t, "11-uplink.netdev", cfgs[2].Name)
	require.Equal(t, "[NetDev]\nName=uplink\nKind=veth\nMACAddress=aa:bb:cc:dd:ee:e1\nMTUBytes=9000\n\n[Peer]\nName=vtuplink\n", cfgs[2].Contents)
	require.Equal(t, "12-management.netdev", cfgs[3].Name)
	require.Equal(t, "[NetDev]\nName=management\nKind=vlan\nMTUBytes=1500\n\n[VLAN]\nId=10\n", cfgs[3].Contents)
}

func TestNetworkFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg api.SystemNetworkConfig

	// Test first config .network file generation.
	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 15)
	require.Equal(t, "20-san1.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=san1\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=both\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nAddress=10.0.101.10/24\nAddress=fd40:1234:1234:101::10/64\nIPv6AcceptRA=false\n", cfgs[0].Contents)
	require.Equal(t, "20-vtsan1.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=vtsan1\n\n[Network]\nBridge=brsan1\n", cfgs[1].Contents)
	require.Equal(t, "20-enaabbccddee01.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee01\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBridge=brsan1\n", cfgs[2].Contents)
	require.Equal(t, "20-brsan1.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=brsan1\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[3].Contents)
	require.Equal(t, "20-san2.network", cfgs[4].Name)
	require.Equal(t, "[Match]\nName=san2\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=both\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nAddress=10.0.102.10/24\nAddress=fd40:1234:1234:102::10/64\nIPv6AcceptRA=false\n", cfgs[4].Contents)
	require.Equal(t, "20-vtsan2.network", cfgs[5].Name)
	require.Equal(t, "[Match]\nName=vtsan2\n\n[Network]\nBridge=brsan2\n\n[BridgeVLAN]\nVLAN=10\n", cfgs[5].Contents)
	require.Equal(t, "20-enaabbccddee02.network", cfgs[6].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee02\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBridge=brsan2\n\n[BridgeVLAN]\nVLAN=10\n", cfgs[6].Contents)
	require.Equal(t, "20-brsan2.network", cfgs[7].Name)
	require.Equal(t, "[Match]\nName=brsan2\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[7].Contents)
	require.Equal(t, "21-management.network", cfgs[8].Name)
	require.Equal(t, "[Match]\nName=management\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=any\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nVLAN=uplink\nLinkLocalAddressing=ipv6\nAddress=10.0.100.10/24\nAddress=fd40:1234:1234:100::10/64\nIPv6AcceptRA=false\n\n[Route]\nGateway=10.0.100.1\nDestination=0.0.0.0/0\n\n[Route]\nGateway=fd40:1234:1234:100::1\nDestination=::/0\n", cfgs[8].Contents)
	require.Equal(t, "21-vtmanagement.network", cfgs[9].Name)
	require.Equal(t, "[Match]\nName=vtmanagement\n\n[Network]\nBridge=brmanagement\n\n[BridgeVLAN]\nPVID=1234\nEgressUntagged=1234\n\n[BridgeVLAN]\nVLAN=100\n\n[BridgeVLAN]\nVLAN=1234\n", cfgs[9].Contents)
	require.Equal(t, "21-bnmanagement.network", cfgs[10].Name)
	require.Equal(t, "[Match]\nName=bnmanagement\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\nBridge=brmanagement\n\n[BridgeVLAN]\nPVID=1234\nEgressUntagged=1234\n\n[BridgeVLAN]\nVLAN=100\n\n[BridgeVLAN]\nVLAN=1234\n", cfgs[10].Contents)
	require.Equal(t, "21-brmanagement.network", cfgs[11].Name)
	require.Equal(t, "[Match]\nName=brmanagement\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[11].Contents)
	require.Equal(t, "21-bnmanagement-dev0.network", cfgs[12].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee03\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBond=bnmanagement\n", cfgs[12].Contents)
	require.Equal(t, "21-bnmanagement-dev1.network", cfgs[13].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee04\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBond=bnmanagement\n", cfgs[13].Contents)
	require.Equal(t, "22-uplink.network", cfgs[14].Name)
	require.Equal(t, "[Match]\nName=uplink\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=ipv4\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=false\nDHCP=ipv4\n\n[Route]\nGateway=_dhcp4\nDestination=0.0.0.0/0\n", cfgs[14].Contents)

	// Test second config .network file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 4)
	require.Equal(t, "20-management.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=management\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=ipv6\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=true\nDHCP=ipv4\n\n[Route]\nGateway=_dhcp4\nDestination=0.0.0.0/0\n\n[Route]\nGateway=_ipv6ra\nDestination=::/0\n", cfgs[0].Contents)
	require.Equal(t, "20-vtmanagement.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=vtmanagement\n\n[Network]\nBridge=brmanagement\n", cfgs[1].Contents)
	require.Equal(t, "20-enaabbccddee01.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=enaabbccddee01\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBridge=brmanagement\n", cfgs[2].Contents)
	require.Equal(t, "20-brmanagement.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=brmanagement\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[3].Contents)

	// Test third config .network file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 4)
	require.Equal(t, "20-eth0.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=eth0\n\n[Link]\nRequiredForOnline=no\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nDomains=example.org\nDNS=ns1.example.org\nDNS=ns2.example.org\nNTP=pool.ntp.example.org\nNTP=10.10.10.10\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=false\nDHCP=ipv4\n", cfgs[0].Contents)
	require.Equal(t, "20-vteth0.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=vteth0\n\n[Network]\nBridge=breth0\n", cfgs[1].Contents)
	require.Equal(t, "20-enffeeddccbbaa.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=enffeeddccbbaa\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBridge=breth0\n", cfgs[2].Contents)
	require.Equal(t, "20-breth0.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=breth0\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[3].Contents)

	// Test fourth config .network file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig4), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 7)
	require.Equal(t, "21-uplink.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=uplink\n\n[Link]\nRequiredForOnline=no\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nVLAN=management\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\nIPv6AcceptRA=false\n", cfgs[0].Contents)
	require.Equal(t, "21-vtuplink.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=vtuplink\n\n[Network]\nBridge=bruplink\n\n[BridgeVLAN]\nPVID=10\nEgressUntagged=10\n\n[BridgeVLAN]\nVLAN=10\n", cfgs[1].Contents)
	require.Equal(t, "21-bnuplink.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=bnuplink\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\nBridge=bruplink\n\n[BridgeVLAN]\nPVID=10\nEgressUntagged=10\n\n[BridgeVLAN]\nVLAN=10\n", cfgs[2].Contents)
	require.Equal(t, "21-bruplink.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=bruplink\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[3].Contents)
	require.Equal(t, "21-bnuplink-dev0.network", cfgs[4].Name)
	require.Equal(t, "[Match]\nName=enaabbccddeee1\n\n[Network]\nLLDP=true\nEmitLLDP=true\nBond=bnuplink\n", cfgs[4].Contents)
	require.Equal(t, "21-bnuplink-dev1.network", cfgs[5].Name)
	require.Equal(t, "[Match]\nName=enaabbccddeee2\n\n[Network]\nLLDP=true\nEmitLLDP=true\nBond=bnuplink\n", cfgs[5].Contents)
	require.Equal(t, "22-management.network", cfgs[6].Name)
	require.Equal(t, "[Match]\nName=management\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=both\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=true\nDHCP=ipv4\n", cfgs[6].Contents)
}
