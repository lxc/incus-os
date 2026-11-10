# Network

IncusOS supports complex network configurations consisting of interfaces, bonds, VLANs and WireGuard. By default, IncusOS will configure each discovered interface to automatically acquire IPv4/IPv6 addresses, DNS, and NTP information from the local network. More complex network setups can be configured via an [install seed](../seed.md), or post-install via the network API.

Before applying any new/updated network configuration, basic validation checks are performed. If this check fails, or the network fails to come up properly as reported by `systemd-networkd`, the changes will be reverted to minimize the chance of accidentally knocking the IncusOS system offline.

Optionally, if the `confirmation_timeout` field is defined when applying a configuration update, IncusOS will automatically roll back the network configuration to the prior state if a followup confirmation command isn't received before the timeout expires. This can be a useful way to test tricky or unknown network configurations that might otherwise break IncusOS' networking configuration.

Be aware that changing network configuration may result in a brief period of time when the system is unreachable over the network.

```{note}
IncusOS automatically configures each interface and bond as a network bridge. This allows for easy out-of-the-box configuration of bridged NICs for containers and virtual machines.
```

## Roles

Each interface, bond, VLAN or WireGuard can be assigned one or more _roles_, which are used by IncusOS to control how the network device is used:

* `cluster`: The device is used for internal cluster communication
* `instances`: The device should be made available for use by Incus containers or virtual machines
* `management`: The device is used for management
* `storage`: The device is used for network-attached storage connectivity

IncusOS will automatically assign the `cluster` and `management` roles to all interfaces if no roles have been manually configured.
If only the `management` role has been assigned, then the `cluster` role will automatically be assigned to the same interfaces.

## Configuration options

Interfaces, bonds, VLANs and WireGuard have a significant number of fields, which are largely self-descriptive and can be viewed in the [API definition](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_network.go).

### Specifying hardware addresses (MACs)

One special feature of note is the handling of hardware addresses (MACs). Both interfaces and bonds associate their configuration with the hardware address, which can be specified in two ways:

* Raw MAC: Specify the hardware address directly, such as `10:66:6a:e5:6a:1c`.

* Interface name: If an interface name is provided, such as `enp5s0`, at startup IncusOS will attempt to get its MAC address and substitute that value in the configuration. This is useful when installing IncusOS across multiple physically identical servers with only a single [install seed](../seed.md).

### Top-level configuration options

The following top-level network configuration options can be set:

* `confirmation_timeout`: If defined, will trigger an automatic roll back of the network configuration unless a followup confirmation command is received before the timeout expires.

* `default_confirmation_timeout`: Default value for `confirmation_timeout`. This will be used if no `confirmation_timeout` provided.

* `ignore_service_checks`: If true, don't wait for DNS and NTP to be configured and available; most systems shouldn't need to set this.

* `interfaces`: Zero or more interfaces that should be configured for the system.

* `bonds`: Zero or more bonds that should be configured for the system.

* `vlans`: Zero or more VLANs that should be configured for the system.

* `wireguard`: Zero or more WireGuard interfaces that should be configured for the system.

* `dns`: Optionally, configure custom DNS information for the system.

* `proxy`: Optionally, configure a proxy for the system.

* `time`: Optionally, configure custom NTP server(s) and timezone for the system.

### Bridge options

Interfaces and bonds accept a `bridge` option to tune the bridge that IncusOS automatically creates on top of them:

* `disable_multicast_snooping`: If `true`, disable multicast snooping on the bridge.

* `stp`: If `true`, enable the Spanning Tree Protocol on the bridge.

### Ethernet options

Interfaces and bonds accept an `ethernet` option to configure offloading and other Ethernet-level features of the underlying physical devices, which for a bond are all of its members:

* `disable_energy_efficient`: If `true`, disable Energy Efficient Ethernet.

* `disable_gro`: If `true`, disable generic receive offload.

* `disable_gso`: If `true`, disable generic segmentation offload.

* `disable_ipv4_tso`: If `true`, disable TCP segmentation offload for IPv4.

* `disable_ipv6_tso`: If `true`, disable TCP segmentation offload for IPv6.

* `rx_buffer_size`, `rx_jumbo_buffer_size`, `rx_mini_buffer_size` and `tx_buffer_size`: The number of entries in the matching ring buffer, either a positive number or `max` to use the largest size supported by the device.

* `wakeonlan`: If `true`, enable Wake-on-LAN on the device.

* `wakeonlan_modes`: A list of Wake-on-LAN modes to enable, defaults to `magic`. For the supported values, refer to systemd's [`WakeOnLan` link configuration option](https://www.freedesktop.org/software/systemd/man/latest/systemd.link.html#WakeOnLan=).

* `wakeonlan_password`: The password to use with the `secureon` Wake-on-LAN mode, in the same format as a hardware address.

### Bond options

On top of its `mode` and `members`, a bond accepts an `options` field to tune its behavior. All intervals and delays are in milliseconds:

* `transmit_hash_policy`: The policy used to select the member for outgoing traffic, one of `layer2`, `layer2+3`, `layer3+4`, `encap2+3`, `encap3+4` or `vlan+srcmac`. Defaults to `layer3+4` for `802.3ad` bonds.

* `lacp_rate`: The rate at which LACP control packets are requested from the peer, either `slow` or `fast`. Defaults to `fast` for `802.3ad` bonds.

* `mii_monitor_interval`: The interval at which the link state of the members is checked. Defaults to `100` unless ARP monitoring is configured.

* `up_delay` and `down_delay`: The delay before a member is enabled or disabled after a link state change. Only used with link state monitoring.

* `arp_interval`: The interval at which ARP requests are sent to check the members. Setting it replaces the link state monitoring with ARP monitoring.

* `arp_ip_targets`: A list of IP addresses to send the ARP requests to when ARP monitoring is used.

### VLAN tags

The bridge created on top of an interface or bond only lets through traffic without a VLAN tag and the VLANs defined on top of it. Additional VLAN IDs can be allowed by listing them in the `vlan_tags` option of the interface or bond, making them available to instances attached to the bridge.

### Strict hardware address filtering

Interfaces accept a `strict_hwaddr` option. If `true`, IncusOS only lets traffic using the interface's own hardware address go out through the physical device. This is useful on networks that don't tolerate additional hardware addresses on a port, but it prevents instances bridged to that interface from reaching the network.

### `required_for_online` values

Network interfaces, bonds, VLANs, and WireGuard interfaces can optionally be configured with the `required_for_online` option that IncusOS will use to determine when that network device is online. Valid values include `ipv4`, `ipv6`, `both`, `any`, and `no`. If not specified, defaults to `any`. For further details, refer to systemd's [`RequiredFamilyForOnline` networkctl configuration option](https://www.freedesktop.org/software/systemd/man/latest/systemd.network.html#RequiredFamilyForOnline=).

### Firewall

IncusOS supports a basic ingress firewall on its interfaces.
This is done by setting the `firewall_rules` option to a list of rules, each of which consists of:

* `action`: What to do with matching traffic, one of `accept`, `drop` or `reject`.

* `source`: Optional; the source IP address or subnet to match.

* `protocol`: Optional; the protocol to match, either `tcp` or `udp`. Must be set together with `port`.

* `port`: Optional; the destination port to match. Must be set together with `protocol`.

On top of the user provided rules, IncusOS will always allow a subset of basic rules (`icmp`, `icmpv6` and established connections).

### Routing

IncusOS never routes traffic between its own interfaces (interfaces, bonds, VLANs and WireGuard).
Routing to and from other interfaces remains possible, allowing IncusOS to act as a gateway for Incus managed networks as well as run VPN services like Tailscale or NetBird as an exit node or subnet router.

### Examples

#### Addressing

Configure two network interfaces, one with IPv4 and the other with IPv6:

```yaml
config:
  interfaces:
  - name: "ip4iface"
    hwaddr: "enp5s0"
    addresses:
    - "dhcp4"

  - name: "ip6iface"
    hwaddr: "enp6s0"
    addresses:
    - "slaac"
```

Configure a network interface with two static IP addresses. When specifying a static IP address, it must include a CIDR mask.

```yaml
config:
  interfaces:
  - name: "enp5s0"
    hwaddr: "enp5s0"

    addresses:
    - "10.234.136.100/24"
    - "fd42:3cfb:8972:3990::100/64"

    routes:
    - to: "0.0.0.0/0"
      via: "10.234.136.1"
    - to: "::/0"
      via: "fd42:3cfb:8972:3990::1"

  dns:
    nameservers:
    - "10.234.136.1"
```

When the gateway is outside the configured address prefix, set `onlink` to `true` to mark the gateway as directly reachable through the interface:

```yaml
config:
  interfaces:
  - name: "enp5s0"
    hwaddr: "enp5s0"
    addresses:
    - "203.0.113.10/32"
    routes:
    - to: "0.0.0.0/0"
      via: "198.51.100.1"
      onlink: true
```

This generates a `GatewayOnLink=yes` route for `systemd-networkd`.

#### Automatic roll back of network configuration

When applying a complex network configuration update, it can be useful to automatically roll back the changes if something goes wrong. IncusOS supports this via the `confirmation_timeout` configuration field.

For example, the following configuration will automatically roll back to the current network configuration after five minutes, unless a followup confirmation command is received:

```yaml
config:
  confirmation_timeout: 5m
  interfaces:
  - addresses:
    - dhcp4
    hwaddr: 10:66:6a:f1:49:aa
    name: enp5s0
    required_for_online: "ipv4"
  time:
    timezone: UTC
```

After applying the network configuration, if IncusOS remains reachable on the network as expected, run `incus admin os system network confirm` before five minutes elapses to confirm and save the new configuration. If something went wrong and IncusOS is no longer available on the network, simply wait the five minutes and IncusOS will re-configure itself with the prior configuration that had been working.

If we want a default confirmation timeout value without having to provide `confirmation_timeout` every single time, we can use the following configuration:

```yaml
config:
  default_confirmation_timeout: 5m
  interfaces:
  - addresses:
    - dhcp4
    hwaddr: 10:66:6a:f1:49:aa
    name: enp5s0
    required_for_online: "ipv4"
  time:
    timezone: UTC
```

Note that the `default_confirmation_timeout` will be effective *next time* you make changes to the network configuration and if there is `default_confirmation_timeout` set for the running configuration, the running state value will be honored and the `default_confirmation_timeout` will be honored next time.

Leaving this value out removes the `default_confirmation_timeout`.

#### VLANs

Configure a VLAN with ID 123 on top of an active-backup bond composed of two interfaces with MTU of 9000 and LLDP enabled:

```yaml
config:
  bonds:
  - name: "management"
    mode: "active-backup"
    mtu: 9000
    lldp: true

    members:
    - "enp5s0"
    - "enp6s0"

    roles:
    - "management"
    - "instances"

  vlans:
  - name: "uplink"
    parent: "management"
    id: 123

    addresses:
    - "dhcp4"
    - "slaac"
```

#### WireGuard

Configure a WireGuard interface with two peers (providing a `private_key` is optional and will be created if empty). Each peer also accepts an optional base64-encoded `preshared_key`:

```yaml
config:
  wireguard:
  - name: "wg0"
    port: 51820
    private_key: "AE1SCwtkp8ruDYlUa9x9wsoTzEOePl3P9sMdFFa9PmI="

    addresses:
    - "10.234.234.100/24"
    - "fd42:3cfb:8972:abcd::100/64"

    routes:
    - to: "10.234.110.0/24"
      via: "10.234.234.110"

    peers:
    - allowed_ips:
      - "10.234.234.110/24"
      - "fd42:3cfb:8972:abcd::110/64"
      - "10.234.110.0/24"
      endpoint: "10.102.89.110:51820"
      public_key: "rJhRcAtHUldTAA/J+TPQPQpr6G9C2Arf5FiTVwjOYCE="

    - allowed_ips:
      - "10.234.234.120/24"
      - "fd42:3cfb:8972:abcd::120/64"
      persistent_keepalive: 30
      public_key: "qPYSgwaJe0VZb4M8smTPpd2rfKHz0X0ypq54ZY4ATVQ="
```

A default route (`0.0.0.0/0` or `::/0`) can be set on a WireGuard interface to send all traffic through the tunnel. IncusOS then uses policy routing to keep the tunnel's own traffic to its peers on the regular routes, and more specific routes on other interfaces remain in use.

#### DNS, NTP, Timezone

```{note}
Changing the system's timezone will take effect immediately, but won't be reflected in the IncusOS terminal UI until after a system reboot because of a [known issue](https://github.com/golang/go/issues/28020) in the Go `time` package.
```

```{note}
Please use name servers that support DNS over TLS when `dns_over_tls` is true
```

Configure custom DNS, NTP, and timezone for IncusOS:

```yaml
config:
  dns:
    hostname: "server01"
    domain: "example.com"

    search_domains:
    - "example.com"
    - "example.org"

    nameservers:
    - "ns1.example.com"
    - "ns2.example.com"
    dns_over_tls: false

  time:
    ntp_servers:
    - "ntp.example.com"

    timezone: "America/New_York"
```

To manually flush the DNS cache at any time, run:

```
incus admin os system network flush-dns
```

#### Proxy

Configure a simple anonymous HTTP(S) proxy for IncusOS:

```yaml
config:
  proxy:
    servers:
      example-proxy:
        host: "proxy.example.com:8080"
        auth: "anonymous"
```

Configure an authenticated HTTP(S) proxy with an exception for `*.example.com` and a total blocking of `*.bad-domain.hacker` for IncusOS:

```yaml
config:
  proxy:
    servers:
      example-proxy:
        host: "proxy.example.com:8080"
        use_tls: true
        auth: "basic"
        username: "myuser"
        password: "mypassword"

    rules:
    - destination: "*.example.com|example.com"
      target: "direct"

    - destination: "*.bad-domain.hacker|bad-domain.hacker"
      target: "none"

    - destination: "*"
      target: "example-proxy"
```

Configure an authenticated HTTP(S) proxy that relies on Kerberos authentication for IncusOS:

```yaml
config:
  proxy:
    servers:
      example-proxy:
        host: "proxy.example.com:8080"
        use_tls: true
        auth: "kerberos"
        realm: "auth.example.com"
        username: "myuser"
        password: "mypassword"
```
