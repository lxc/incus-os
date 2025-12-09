# Network

IncusOS supports complex network configurations consisting of interfaces, bonds, and VLANs. By default, IncusOS will configure each discovered interface to automatically acquire IPv4/IPv6 addresses, DNS, and NTP information from the local network. More complex network setups can be configured via an [install seed](../seed.md), or post-install via the network API.

Before applying any new/updated network configuration, basic validation checks are performed. If this check fails, or the network fails to come up properly as reported by `systemd-networkd`, the changes will be reverted to minimize the chance of accidentally knocking the IncusOS system offline.

Be aware that changing network configuration may result in a brief period of time when the system is unreachable over the network.

```{note}
IncusOS automatically configures each interface and bond as a network bridge. This allows for easy out-of-the-box configuration of bridged NICs for containers and virtual machines.
```

## Roles

Each interface, bond or VLAN can be assigned one or more _roles_, which are used by IncusOS to control how the network device is used:

* `cluster`: The device is used for internal cluster communication
* `instances`: The device should be made available for use by Incus containers or virtual machines
* `management`: The device is used for management
* `storage`: The device is used for network-attached storage connectivity

By default, the `cluster` and `management` roles will be assigned.

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

* `proxy`: Optionally, configure a proxy for the system.

* `time`: Optionally, configure custom NTP server(s) and timezone for the system.

## Firewall

IncusOS supports a basic ingress firewall on its interfaces.
This is done by setting the `firewall_rules` option to a list of rules (action, source address, protocol and port).

On top of the user provided rules, IncusOS will always allow a subset of basic rules (`icmp`, `icmpv6` and established connections).

### Examples

#### Addressing

Configure two network interfaces, one with IPv4 and the other with IPv6:

```
{
  "config": {
    "interfaces": [
      {
        "name": "ip4iface",
        "hwaddr": "enp5s0",
        "addresses": [
          "dhcp4"
        ]
      },
      {
        "name": "ip6iface",
        "hwaddr": "enp6s0",
        "addresses": [
          "slaac"
        ]
      }
    ]
  }
}
```

Configure a network interface with two static IP addresses. When specifying a static IP address, it must include a CIDR mask.

```
{
  "config": {
    "interfaces": [
      {
        "name": "enp5s0",
        "hwaddr": "enp5s0",
        "addresses": [
          "10.234.136.100/24",
          "fd42:3cfb:8972:3990::100/64"
        ],
        "routes": [
          {
            "to": "0.0.0.0/0",
            "via": "10.234.136.1"
          },
          {
            "to": "::/0",
            "via": "fd42:3cfb:8972:3990::1"
          }
        ]
      }
    ],
    "dns": {
      "nameservers": [
        "10.234.136.1"
      ]
    }
  }
}
```

#### VLANs

Configure a VLAN with ID 123 on top of an active-backup bond composed of two interfaces with MTU of 9000 and LLDP enabled:

```
{
  "config": {
    "bonds": [
      {
        "name": "management",
        "mode": "active-backup",
        "mtu": 9000,
        "lldp": true,
        "members": [
          "enp5s0",
          "enp6s0"
        ],
        "roles": [
          "management",
          "interfaces"
        ]
      }
    ],
    "vlans": [
      {
        "name": "uplink",
        "parent": "management",
        "id": 123,
        "addresses": [
          "dhcp4",
          "slaac"
        ]
      }
    ]
  }
}
```

#### DNS, NTP, Timezone

Configure custom DNS, NTP, and timezone for IncusOS:

```
{
  "config": {
    "dns": {
      "hostname": "server01",
      "domain": "example.com",
      "search_domains": [
        "example.com",
        "example.org"
      ],
      "nameservers": [
        "ns1.example.com",
        "ns2.example.com"
      ]
    },
    "time": {
      "ntp_servers": [
        "ntp.example.com"
      ],
      "timezone": "America/New_York"
    }
  }
}
```

#### Proxy

Configure a simple anonymous HTTP(S) proxy for IncusOS:

```
{
  "config": {
    "proxy": {
      "servers": {
        "example-proxy": {
          "host": "proxy.example.com:8080",
          "auth": "anonymous"
        }
      }
    }
  }
}
```

Configure an authenticated HTTP(S) proxy with an exception for `*.example.com` and a total blocking of `*.bad-domain.hacker` for IncusOS:

```
{
  "config": {
    "proxy": {
      "servers": {
        "example-proxy": {
          "host": "proxy.example.com:8080",
          "use_tls": true,
          "auth": "basic",
          "username": "myuser",
          "password": "mypassword"
        }
      },
      "rules": [
        {
          "destination": "*.example.com|example.com",
          "target": "direct"
        },
        {
          "destination": "*.bad-domain.hacker|bad-domain.hacker",
          "target": "none"
        },
        {
          "destination": "*",
          "target": "example-proxy"
        }
      ]
    }
  }
}
```

Configure an authenticated HTTP(S) proxy that relies on Kerberos authentication for IncusOS:

```
{
  "config": {
    "proxy": {
      "servers": {
        "example-proxy": {
          "host": "proxy.example.com:8080",
          "use_tls": true,
          "auth": "kerberos",
          "realm": "auth.example.com",
          "username": "myuser",
          "password": "mypassword"
        }
      }
    }
  }
}
```
