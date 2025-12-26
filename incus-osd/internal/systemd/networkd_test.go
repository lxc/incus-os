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
      - cluster

wireguard:
  - addresses:
    - 10.9.0.7/24
    - fd25:6c9a:6c19::7/64
    name: wg0
    peers:
    - allowed_ips:
      - 10.9.0.1/32
      - fd25:6c9a:6c19::1/128
      - 192.168.1.0/24
      endpoint: 10.102.89.87:51820
      public_key: rJhRcAtHUldTAA/J+TPQPQpr6G9C2Arf5FiTVwjOYCE=
    - allowed_ips:
      - 10.9.0.3/32
      - fd25:6c9a:6c19::3/128
      endpoint: 10.180.60.231:51820
      public_key: qPYSgwaJe0VZb4M8smTPpd2rfKHz0X0ypq54ZY4ATVQ=
    port: 51820
    private_key: AE1SCwtkp8ruDYlUa9x9wsoTzEOePl3P9sMdFFa9PmI=
    roles:
      - management
    routes:
      - to: 192.168.2.0/24
        via: 10.9.0.3
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

wireguard:
  - addresses:
    - 10.9.0.7/24
    - fd25:6c9a:6c19::7/64
    name: wg0
    mtu: 1420
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
time:
  ntp_servers:
    - pool.ntp.example.org
    - 10.10.10.10
proxy:
  servers:
    example:
      host: https://proxy.example.org
      auth: anonymous
interfaces:
  - name: enxffeeddccbbaa
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

var networkdConfig5 = `
interfaces:
  - name: no-hw-tso-gro-gso
    addresses:
      - 10.0.101.10/24
      - fd40:1234:1234:101::10/64
    required_for_online: both
    hwaddr: AA:BB:CC:DD:EE:01
    ethernet:
      disable_energy_efficient: true
      disable_ipv4_tso: true
      disable_ipv6_tso: true
      disable_gro: true
      disable_gso: true
      wakeonlan: true
      wakeonlan_modes:
      - magic
      - secureon
      wakeonlan_password: 11:22:33:44:55:66

bonds:
  - name: no-hw-tso
    mode: 802.3ad
    mtu: 9000
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
      - AA:BB:CC:DD:EE:02
      - AA:BB:CC:DD:EE:03
    ethernet:
      disable_energy_efficient: true
      disable_ipv4_tso: true
      disable_ipv6_tso: true
`

var badNetworkdConfig1 = `
interfaces:
  - name: myreallylongname
    addresses:
      - dhcp4
    hwaddr: eth0
`

var badNetworkdConfig2 = `
interfaces:
  - name: _reserved
    addresses:
      - dhcp4
    hwaddr: eth0
`

var badNetworkdConfig3 = `
interfaces:
  - name: iface
    addresses:
      - dhcp4
    hwaddr: eth0
  - name: iface
    addresses:
      - dhcp4
    hwaddr: eth0
`

var badNetworkdConfig4 = `
interfaces:
  - name: eth0
    addresses:
      - 192.168.0.100
    hwaddr: eth0
`

var badNetworkdConfig5 = `
wireguard:
  - name: wg0
    private_key: invalidkey
    addresses:
      - 192.168.0.100/24
`

var badNetworkdConfig6 = `
wireguard:
  - name: wg0
    port: 65536
    addresses:
      - 192.168.0.100/24
`

func TestBadNetworkConfig(t *testing.T) {
	t.Parallel()

	{
		var cfg api.SystemNetworkConfig

		err := yaml.Unmarshal([]byte(badNetworkdConfig1), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, false)
		require.EqualError(t, err, "interface 0 name 'myreallylongname' cannot be longer than 13 characters")
	}

	{
		var cfg api.SystemNetworkConfig

		err := yaml.Unmarshal([]byte(badNetworkdConfig2), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, false)
		require.EqualError(t, err, "interface 0 name cannot begin with an underscore")
	}

	{
		var cfg api.SystemNetworkConfig

		err := yaml.Unmarshal([]byte(badNetworkdConfig3), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, false)
		require.EqualError(t, err, "duplicate interface/bond/vlan/wireguard name: iface")
	}

	{
		var cfg api.SystemNetworkConfig

		err := yaml.Unmarshal([]byte(badNetworkdConfig4), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, false)
		require.EqualError(t, err, "interface 0 address 0 invalid IP address '192.168.0.100', must provide a CIDR mask")
	}

	{
		var cfg api.SystemNetworkConfig

		err := yaml.Unmarshal([]byte(badNetworkdConfig5), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, false)
		require.EqualError(t, err, "wireguard 0 private key 'invalidkey' invalid")
	}

	{
		var cfg api.SystemNetworkConfig

		err := yaml.Unmarshal([]byte(badNetworkdConfig6), &cfg)
		require.NoError(t, err)

		err = ValidateNetworkConfiguration(&cfg, false)
		require.EqualError(t, err, "wireguard 0 port '65536' out of range")
	}
}

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
		require.Equal(t, "cluster", cfg.VLANs[0].Roles[0])
		require.Len(t, cfg.Wireguard, 1)
		require.Equal(t, "wg0", cfg.Wireguard[0].Name)
		require.Len(t, cfg.Wireguard[0].Addresses, 2)
		require.Equal(t, "fd25:6c9a:6c19::7/64", cfg.Wireguard[0].Addresses[1])
		require.Equal(t, "AE1SCwtkp8ruDYlUa9x9wsoTzEOePl3P9sMdFFa9PmI=", cfg.Wireguard[0].PrivateKey)
		require.Equal(t, 51820, cfg.Wireguard[0].Port)
		require.Equal(t, "management", cfg.Wireguard[0].Roles[0])
		require.Len(t, cfg.Wireguard[0].Peers, 2)
		require.Len(t, cfg.Wireguard[0].Peers[0].AllowedIPs, 3)
		require.Equal(t, "fd25:6c9a:6c19::1/128", cfg.Wireguard[0].Peers[0].AllowedIPs[1])
		require.Len(t, cfg.Wireguard[0].Peers[1].AllowedIPs, 2)
		require.Equal(t, "10.180.60.231:51820", cfg.Wireguard[0].Peers[1].Endpoint)
		require.Equal(t, "qPYSgwaJe0VZb4M8smTPpd2rfKHz0X0ypq54ZY4ATVQ=", cfg.Wireguard[0].Peers[1].PublicKey)

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
		require.Len(t, cfg.Wireguard, 1)
		require.Equal(t, "wg0", cfg.Wireguard[0].Name)
		require.Equal(t, 1420, cfg.Wireguard[0].MTU)
		// key is empty as test doesn't call ApplyNetworkConfiguration
		require.Empty(t, cfg.Wireguard[0].PrivateKey)

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
		require.Len(t, cfg.Time.NTPServers, 2)
		require.Equal(t, "pool.ntp.example.org", cfg.Time.NTPServers[0])
		require.Equal(t, "10.10.10.10", cfg.Time.NTPServers[1])
		require.Len(t, cfg.Proxy.Servers, 1)
		require.Equal(t, "https://proxy.example.org", cfg.Proxy.Servers["example"].Host)
		require.Equal(t, "anonymous", cfg.Proxy.Servers["example"].Auth)

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
	require.Equal(t, "00-_paabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:01\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=_paabbccddee01\n", cfgs[0].Contents)
	require.Equal(t, "00-_paabbccddee02.link", cfgs[1].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:02\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=_paabbccddee02\n", cfgs[1].Contents)
	require.Equal(t, "01-_paabbccddee03.link", cfgs[2].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:03\n\n[Link]\nNamePolicy=\nName=_paabbccddee03\n", cfgs[2].Contents)
	require.Equal(t, "01-_paabbccddee04.link", cfgs[3].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:04\n\n[Link]\nNamePolicy=\nName=_paabbccddee04\n", cfgs[3].Contents)

	// Test second config .link file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-_paabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:01\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=_paabbccddee01\n", cfgs[0].Contents)

	// Test third config .link file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 1)
	require.Equal(t, "00-_pffeeddccbbaa.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=FF:EE:DD:CC:BB:AA\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=_pffeeddccbbaa\n", cfgs[0].Contents)

	// Test fourth config .link file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig4), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "01-_paabbccddeee1.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=aa:bb:cc:dd:ee:e1\n\n[Link]\nNamePolicy=\nName=_paabbccddeee1\n", cfgs[0].Contents)
	require.Equal(t, "01-_paabbccddeee2.link", cfgs[1].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=aa:bb:cc:dd:ee:e2\n\n[Link]\nNamePolicy=\nName=_paabbccddeee2\n", cfgs[1].Contents)

	// Test fifth config .link file genration
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig5), &networkCfg)
	require.NoError(t, err)

	cfgs = generateLinkFileContents(networkCfg)
	require.Len(t, cfgs, 3)
	require.Equal(t, "00-_paabbccddee01.link", cfgs[0].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:01\n\n[Link]\nMACAddressPolicy=random\nNamePolicy=\nName=_paabbccddee01\nGenericSegmentationOffload=false\nGenericReceiveOffload=false\nTCPSegmentationOffload=false\nTCP6SegmentationOffload=false\nWakeOnLan=magic\nWakeOnLan=secureon\nWakeOnLanPassword=11:22:33:44:55:66\n[EnergyEfficientEthernet]\nEnable=false\n", cfgs[0].Contents)
	require.Equal(t, "01-_paabbccddee02.link", cfgs[1].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:02\n\n[Link]\nNamePolicy=\nName=_paabbccddee02\nTCPSegmentationOffload=false\nTCP6SegmentationOffload=false\n[EnergyEfficientEthernet]\nEnable=false\n", cfgs[1].Contents)
	require.Equal(t, "01-_paabbccddee03.link", cfgs[2].Name)
	require.Equal(t, "[Match]\nPermanentMACAddress=AA:BB:CC:DD:EE:03\n\n[Link]\nNamePolicy=\nName=_paabbccddee03\nTCPSegmentationOffload=false\nTCP6SegmentationOffload=false\n[EnergyEfficientEthernet]\nEnable=false\n", cfgs[2].Contents)
}

func TestNetdevFileGeneration(t *testing.T) {
	t.Parallel()

	var networkCfg api.SystemNetworkConfig

	// Test first config .netdev file generation.
	err := yaml.Unmarshal([]byte(networkdConfig1), &networkCfg)
	require.NoError(t, err)

	cfgs := generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 9)
	require.Equal(t, "10-san1.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=san1\nKind=bridge\n\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
	require.Equal(t, "10-_vsan1.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=_vsan1\nKind=veth\nMACAddress=AA:BB:CC:DD:EE:01\n\n\n[Peer]\nName=_iaabbccddee01\n", cfgs[1].Contents)
	require.Equal(t, "10-san2.netdev", cfgs[2].Name)
	require.Equal(t, "[NetDev]\nName=san2\nKind=bridge\n\n\n[Bridge]\nVLANFiltering=true\n", cfgs[2].Contents)
	require.Equal(t, "10-_vsan2.netdev", cfgs[3].Name)
	require.Equal(t, "[NetDev]\nName=_vsan2\nKind=veth\nMACAddress=AA:BB:CC:DD:EE:02\n\n\n[Peer]\nName=_iaabbccddee02\n", cfgs[3].Contents)
	require.Equal(t, "11-_bmanagement.netdev", cfgs[4].Name)
	require.Equal(t, "[NetDev]\nName=_bmanagement\nKind=bond\nMTUBytes=9000\n\n[Bond]\nMode=802.3ad\n", cfgs[4].Contents)
	require.Equal(t, "11-management.netdev", cfgs[5].Name)
	require.Equal(t, "[NetDev]\nName=management\nKind=bridge\nMTUBytes=9000\n\n[Bridge]\nVLANFiltering=true\n", cfgs[5].Contents)
	require.Equal(t, "11-_vmanagement.netdev", cfgs[6].Name)
	require.Equal(t, "[NetDev]\nName=_vmanagement\nKind=veth\nMACAddress=AA:BB:CC:DD:EE:03\nMTUBytes=9000\n\n[Peer]\nName=_iaabbccddee03\n", cfgs[6].Contents)
	require.Equal(t, "12-uplink.netdev", cfgs[7].Name)
	require.Equal(t, "[NetDev]\nName=uplink\nKind=vlan\nMTUBytes=1500\n\n[VLAN]\nId=1234\n", cfgs[7].Contents)
	require.Equal(t, "13-wg0.netdev", cfgs[8].Name)
	require.Equal(t, "[NetDev]\nName=wg0\nKind=wireguard\n\n\n[WireGuard]\nPrivateKey=AE1SCwtkp8ruDYlUa9x9wsoTzEOePl3P9sMdFFa9PmI=\nListenPort=51820\n\n[WireGuardPeer]\nPublicKey=rJhRcAtHUldTAA/J+TPQPQpr6G9C2Arf5FiTVwjOYCE=\nAllowedIPs=10.9.0.1/32\nAllowedIPs=fd25:6c9a:6c19::1/128\nAllowedIPs=192.168.1.0/24\nEndpoint=10.102.89.87:51820\n\n\n[WireGuardPeer]\nPublicKey=qPYSgwaJe0VZb4M8smTPpd2rfKHz0X0ypq54ZY4ATVQ=\nAllowedIPs=10.9.0.3/32\nAllowedIPs=fd25:6c9a:6c19::3/128\nEndpoint=10.180.60.231:51820\n\n\n", cfgs[8].Contents)

	// Test second config .netdev file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 3)
	require.Equal(t, "10-management.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=management\nKind=bridge\nMTUBytes=9000\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
	require.Equal(t, "10-_vmanagement.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=_vmanagement\nKind=veth\nMACAddress=AA:BB:CC:DD:EE:01\nMTUBytes=9000\n\n[Peer]\nName=_iaabbccddee01\n", cfgs[1].Contents)
	require.Equal(t, "13-wg0.netdev", cfgs[2].Name)
	require.Equal(t, "[NetDev]\nName=wg0\nKind=wireguard\nMTUBytes=1420\n\n[WireGuard]\nPrivateKey=\n\n\n", cfgs[2].Contents)

	// Test third config .netdev file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	// The third test case contains a mock USB NIC name, and we must first validate the config
	// to properly correct that name.
	err = ValidateNetworkConfiguration(&networkCfg, true)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 2)
	require.Equal(t, "10-ffeeddccbbaa.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=ffeeddccbbaa\nKind=bridge\n\n\n[Bridge]\nVLANFiltering=true\n", cfgs[0].Contents)
	require.Equal(t, "10-_vffeeddccbbaa.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=_vffeeddccbbaa\nKind=veth\nMACAddress=FF:EE:DD:CC:BB:AA\n\n\n[Peer]\nName=_iffeeddccbbaa\n", cfgs[1].Contents)

	// Test fourth config .netdev file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig4), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetdevFileContents(networkCfg)
	require.Len(t, cfgs, 4)
	require.Equal(t, "11-_buplink.netdev", cfgs[0].Name)
	require.Equal(t, "[NetDev]\nName=_buplink\nKind=bond\nMTUBytes=9000\n\n[Bond]\nMode=802.3ad\n", cfgs[0].Contents)
	require.Equal(t, "11-uplink.netdev", cfgs[1].Name)
	require.Equal(t, "[NetDev]\nName=uplink\nKind=bridge\nMTUBytes=9000\n\n[Bridge]\nVLANFiltering=true\n", cfgs[1].Contents)
	require.Equal(t, "11-_vuplink.netdev", cfgs[2].Name)
	require.Equal(t, "[NetDev]\nName=_vuplink\nKind=veth\nMACAddress=aa:bb:cc:dd:ee:e1\nMTUBytes=9000\n\n[Peer]\nName=_iaabbccddeee1\n", cfgs[2].Contents)
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
	require.Len(t, cfgs, 16)
	require.Equal(t, "20-_vsan1.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=_vsan1\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=both\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nAddress=10.0.101.10/24\nAddress=fd40:1234:1234:101::10/64\nIPv6AcceptRA=false\n", cfgs[0].Contents)
	require.Equal(t, "20-_iaabbccddee01.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=_iaabbccddee01\n\n[Network]\nBridge=san1\n", cfgs[1].Contents)
	require.Equal(t, "20-_paabbccddee01.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=_paabbccddee01\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBridge=san1\n", cfgs[2].Contents)
	require.Equal(t, "20-san1.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=san1\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[3].Contents)
	require.Equal(t, "20-_vsan2.network", cfgs[4].Name)
	require.Equal(t, "[Match]\nName=_vsan2\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=both\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nAddress=10.0.102.10/24\nAddress=fd40:1234:1234:102::10/64\nIPv6AcceptRA=false\n", cfgs[4].Contents)
	require.Equal(t, "20-_iaabbccddee02.network", cfgs[5].Name)
	require.Equal(t, "[Match]\nName=_iaabbccddee02\n\n[Network]\nBridge=san2\n\n[BridgeVLAN]\nVLAN=10\n", cfgs[5].Contents)
	require.Equal(t, "20-_paabbccddee02.network", cfgs[6].Name)
	require.Equal(t, "[Match]\nName=_paabbccddee02\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBridge=san2\n\n[BridgeVLAN]\nVLAN=10\n", cfgs[6].Contents)
	require.Equal(t, "20-san2.network", cfgs[7].Name)
	require.Equal(t, "[Match]\nName=san2\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[7].Contents)
	require.Equal(t, "21-_vmanagement.network", cfgs[8].Name)
	require.Equal(t, "[Match]\nName=_vmanagement\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=any\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nVLAN=uplink\nLinkLocalAddressing=ipv6\nAddress=10.0.100.10/24\nAddress=fd40:1234:1234:100::10/64\nIPv6AcceptRA=false\n\n[Route]\nGateway=10.0.100.1\nDestination=0.0.0.0/0\n\n[Route]\nGateway=fd40:1234:1234:100::1\nDestination=::/0\n", cfgs[8].Contents)
	require.Equal(t, "21-_iaabbccddee03.network", cfgs[9].Name)
	require.Equal(t, "[Match]\nName=_iaabbccddee03\n\n[Network]\nBridge=management\n\n[BridgeVLAN]\nVLAN=100\n\n[BridgeVLAN]\nVLAN=1234\n", cfgs[9].Contents)
	require.Equal(t, "21-_bmanagement.network", cfgs[10].Name)
	require.Equal(t, "[Match]\nName=_bmanagement\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\nBridge=management\n\n[BridgeVLAN]\nVLAN=100\n\n[BridgeVLAN]\nVLAN=1234\n", cfgs[10].Contents)
	require.Equal(t, "21-management.network", cfgs[11].Name)
	require.Equal(t, "[Match]\nName=management\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[11].Contents)
	require.Equal(t, "21-_bmanagement-dev0.network", cfgs[12].Name)
	require.Equal(t, "[Match]\nName=_paabbccddee03\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBond=_bmanagement\n", cfgs[12].Contents)
	require.Equal(t, "21-_bmanagement-dev1.network", cfgs[13].Name)
	require.Equal(t, "[Match]\nName=_paabbccddee04\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBond=_bmanagement\n", cfgs[13].Contents)
	require.Equal(t, "22-uplink.network", cfgs[14].Name)
	require.Equal(t, "[Match]\nName=uplink\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=ipv4\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=false\nDHCP=ipv4\n\n[Route]\nGateway=_dhcp4\nDestination=0.0.0.0/0\n", cfgs[14].Contents)
	require.Equal(t, "23-wg0.network", cfgs[15].Name)
	require.Equal(t, "[Match]\nName=wg0\n\n[Network]\nLinkLocalAddressing=ipv6\nAddress=10.9.0.7/24\nAddress=fd25:6c9a:6c19::7/64\nIPv6AcceptRA=false\n\n[Route]\nGateway=10.9.0.3\nDestination=192.168.2.0/24\n", cfgs[15].Contents)

	// Test second config .network file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig2), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 5)
	require.Equal(t, "20-_vmanagement.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=_vmanagement\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=ipv6\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=true\nDHCP=ipv4\n\n[Route]\nGateway=_dhcp4\nDestination=0.0.0.0/0\n\n[Route]\nGateway=_ipv6ra\nDestination=::/0\n", cfgs[0].Contents)
	require.Equal(t, "20-_iaabbccddee01.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=_iaabbccddee01\n\n[Network]\nBridge=management\n", cfgs[1].Contents)
	require.Equal(t, "20-_paabbccddee01.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=_paabbccddee01\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBridge=management\n[Link]\nMTUBytes=9000\n", cfgs[2].Contents)
	require.Equal(t, "20-management.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=management\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n[Link]\nMTUBytes=9000\n", cfgs[3].Contents)
	require.Equal(t, "23-wg0.network", cfgs[4].Name)
	require.Equal(t, "[Match]\nName=wg0\n\n[Network]\nLinkLocalAddressing=ipv6\nAddress=10.9.0.7/24\nAddress=fd25:6c9a:6c19::7/64\nIPv6AcceptRA=false\n", cfgs[4].Contents)

	// Test third config .network file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig3), &networkCfg)
	require.NoError(t, err)

	// The third test case contains a mock USB NIC name, and we must first validate the config
	// to properly correct that name.
	err = ValidateNetworkConfiguration(&networkCfg, true)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 4)
	require.Equal(t, "20-_vffeeddccbbaa.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=_vffeeddccbbaa\n\n[Link]\nRequiredForOnline=no\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nDomains=example.org\nDNS=ns1.example.org\nDNS=ns2.example.org\nNTP=pool.ntp.example.org\nNTP=10.10.10.10\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=false\nDHCP=ipv4\n", cfgs[0].Contents)
	require.Equal(t, "20-_iffeeddccbbaa.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=_iffeeddccbbaa\n\n[Network]\nBridge=ffeeddccbbaa\n", cfgs[1].Contents)
	require.Equal(t, "20-_pffeeddccbbaa.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=_pffeeddccbbaa\n\n[Network]\nLLDP=false\nEmitLLDP=false\nBridge=ffeeddccbbaa\n", cfgs[2].Contents)
	require.Equal(t, "20-ffeeddccbbaa.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=ffeeddccbbaa\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[3].Contents)

	// Test fourth config .network file generation.
	networkCfg = api.SystemNetworkConfig{}
	err = yaml.Unmarshal([]byte(networkdConfig4), &networkCfg)
	require.NoError(t, err)

	cfgs = generateNetworkFileContents(networkCfg)
	require.Len(t, cfgs, 7)
	require.Equal(t, "21-_vuplink.network", cfgs[0].Name)
	require.Equal(t, "[Match]\nName=_vuplink\n\n[Link]\nRequiredForOnline=no\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nVLAN=management\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\nIPv6AcceptRA=false\n", cfgs[0].Contents)
	require.Equal(t, "21-_iaabbccddeee1.network", cfgs[1].Name)
	require.Equal(t, "[Match]\nName=_iaabbccddeee1\n\n[Network]\nBridge=uplink\n\n[BridgeVLAN]\nVLAN=10\n", cfgs[1].Contents)
	require.Equal(t, "21-_buplink.network", cfgs[2].Name)
	require.Equal(t, "[Match]\nName=_buplink\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\nBridge=uplink\n\n[BridgeVLAN]\nVLAN=10\n", cfgs[2].Contents)
	require.Equal(t, "21-uplink.network", cfgs[3].Name)
	require.Equal(t, "[Match]\nName=uplink\n\n[Network]\nLinkLocalAddressing=no\nConfigureWithoutCarrier=yes\n", cfgs[3].Contents)
	require.Equal(t, "21-_buplink-dev0.network", cfgs[4].Name)
	require.Equal(t, "[Match]\nName=_paabbccddeee1\n\n[Network]\nLLDP=true\nEmitLLDP=true\nBond=_buplink\n", cfgs[4].Contents)
	require.Equal(t, "21-_buplink-dev1.network", cfgs[5].Name)
	require.Equal(t, "[Match]\nName=_paabbccddeee2\n\n[Network]\nLLDP=true\nEmitLLDP=true\nBond=_buplink\n", cfgs[5].Contents)
	require.Equal(t, "22-management.network", cfgs[6].Name)
	require.Equal(t, "[Match]\nName=management\n\n[Link]\nRequiredForOnline=yes\nRequiredFamilyForOnline=both\n\n[DHCP]\nClientIdentifier=mac\nRouteMetric=100\nUseMTU=true\n\n[Network]\nLinkLocalAddressing=ipv6\nIPv6AcceptRA=true\nDHCP=ipv4\n", cfgs[6].Contents)
}
