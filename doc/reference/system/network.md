# Network

IncusOS supports complex network configurations consisting of interfaces, bonds, and VLANs. By default, IncusOS will configure each discovered interface to automatically acquire IPv4/IPv6 addresses, DNS, and NTP information from the local network. More complex network setups can be configured via an [install seed](../seed.md), or post-install via the network API.

Before applying any new/updated network configuration, basic validation checks are performed. If this check fails, or the network fails to come up properly as reported by `systemd-networkd`, the changes will be reverted to minimize the chance of accidentally knocking the IncusOS system offline.

Be aware that changing network configuration may result in a brief period of time when the system is unreachable over the network.

## Configuration options

Interfaces, bonds, and VLANs have a significant number of fields, which are largely self-descriptive and can be viewed in the [API definition](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_network.go).

One special feature of note is the handling of hardware addresses (MACs). Both interfaces and bonds associate their configuration with the hardware address, which can be specified in two ways:

* Raw MAC: Specify the hardware address directly, such as `10:66:6a:e5:6a:1c`.

* Interface name: If an interface name is provided, such as `enp5s0`, at startup IncusOS will attempt to get its MAC address and substitute that value in the configuration. This is useful when installing IncusOS across multiple physically identical servers with only a single [install seed](../seed.md).

The following configuration options can be set:

* `interfaces`: Zero or more interfaces that should be configured for the system.

* `bonds`: Zero or more bonds that should be configured for the system.

* `vlans`: Zero or more VLANs that should be configured for the system.

* `dns`: Optionally, configure custom DNS information for the system.

* `ntp`: Optionally, configure custom NTP server(s) for the system.

* `proxy`: Optionally, configure a proxy for the system.

### Examples

Configure two network interfaces, one with IPv4 and the other with IPv6:

```
{
    "interfaces": [
        {"name": "ip4iface",
         "hwaddr": "enp5s0",
         "addresses": ["dhcp4"]},
        {"name": "ip6iface",
         "hwaddr": "enp6s0",
         "addresses": ["slaac"]}
    ]
}
```

Configure a network interface with two static IP addresses. When specifying a static IP address, it must include a CIDR mask.

```
{
    "interfaces": [
        {"name": "enp5s0",
         "hwaddr": "enp5s0",
         "addresses": ["10.234.136.100/24", "fd42:3cfb:8972:3990::100/64"],
         "routes": [
             {"to":"0.0.0.0/0", "via":"10.234.136.1"},
             {"to":"::/0", "via":"fd42:3cfb:8972:3990::1"}
         ]}
    ],
    "dns": {
        "nameservers": ["10.234.136.1"]
    }
}
```

Configure a VLAN with ID 123 on top of an active-backup bond composed of two interfaces with MTU of 9000 and LLDP enabled:

```
{
    "bonds": [
        {"name:", "management",
         "mode": "active-backup",
         "mtu": 9000,
         "lldp": true,
         "members": ["enp5s0", "enp6s0"],
         "roles": ["management", "interfaces"]
        }
    ],
    "vlans": [
        {"name": "uplink",
         "parent": "management",
         "id": 123,
         "addresses": ["dhcp4", "slaac"]
        }
    ]
}
```

Configure custom DNS and NTP for IncusOS:

```
{
    "dns": {
        "hostname": "server01",
        "domain": "example.com",
        "search_domains": ["example.com", "example.org"],
        "nameservers": ["ns1.example.com", "ns2.example.com"]
    },
    "ntp": {
        "timeservers": ["ntp.example.com"]
    }
}
```
