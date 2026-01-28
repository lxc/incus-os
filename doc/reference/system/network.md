# Network

IncusOS supports complex network configurations consisting of interfaces, bonds, VLANs and WireGuard. By default, IncusOS will configure each discovered interface to automatically acquire IPv4/IPv6 addresses, DNS, and NTP information from the local network. More complex network setups can be configured via an [install seed](../seed.md), or post-install via the network API.

Before applying any new/updated network configuration, basic validation checks are performed. If this check fails, or the network fails to come up properly as reported by `systemd-networkd`, the changes will be reverted to minimize the chance of accidentally knocking the IncusOS system offline.

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

One special feature of note is the handling of hardware addresses (MACs). Both interfaces and bonds associate their configuration with the hardware address, which can be specified in two ways:

* Raw MAC: Specify the hardware address directly, such as `10:66:6a:e5:6a:1c`.

* Interface name: If an interface name is provided, such as `enp5s0`, at startup IncusOS will attempt to get its MAC address and substitute that value in the configuration. This is useful when installing IncusOS across multiple physically identical servers with only a single [install seed](../seed.md).

The following configuration options can be set:

* `interfaces`: Zero or more interfaces that should be configured for the system.

* `bonds`: Zero or more bonds that should be configured for the system.

* `vlans`: Zero or more VLANs that should be configured for the system.

* `wireguard`: Zero or more WireGuard interfaces that should be configured for the system.

* `dns`: Optionally, configure custom DNS information for the system.

* `proxy`: Optionally, configure a proxy for the system.

* `time`: Optionally, configure custom NTP server(s) and timezone for the system.

### Firewall

IncusOS supports a basic ingress firewall on its interfaces.
This is done by setting the `firewall_rules` option to a list of rules (action, source address, protocol and port).

On top of the user provided rules, IncusOS will always allow a subset of basic rules (`icmp`, `icmpv6` and established connections).

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

Configure a WireGuard interface with two peers (providing a private_key is optional and will be created if empty):

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

#### DNS, NTP, Timezone

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

  time:
    ntp_servers:
    - "ntp.example.com"

    timezone: "America/New_York"
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
