# Directly attach instances to host network

When running [Incus](../reference/applications/incus.md) on IncusOS, by default you will get a NAT'ed bridged network `incusbr0`:

```
gibmat@futurfusion:~$ incus network list
+----------+--------+---------+----------------+---------------------------+----------------------------+---------+---------+
|   NAME   |  TYPE  | MANAGED |      IPV4      |           IPV6            |        DESCRIPTION         | USED BY |  STATE  |
+----------+--------+---------+----------------+---------------------------+----------------------------+---------+---------+
| incusbr0 | bridge | YES     | 10.89.179.1/24 | fd42:e1a1:408f:710a::1/64 | Local network bridge (NAT) | 1       | CREATED |
+----------+--------+---------+----------------+---------------------------+----------------------------+---------+---------+
```

However, sometimes you may want to attach a container or virtual machine directly to the host's network. This is easily accomplished by assigning the `instances` role to the appropriate interfaces or bonds.

First, get the current IncusOS network configuration:

```
gibmat@futurfusion:~$ incus admin os system show network
WARNING: The IncusOS API and configuration is subject to change

config:
  interfaces:
  - addresses:
    - dhcp4
    - slaac
    hwaddr: 10:66:6a:1f:50:b7
    lldp: false
    name: enp5s0
    required_for_online: "no"
state:
  interfaces:
    enp5s0:
      addresses:
      - 10.234.136.193
      - fd42:3cfb:8972:3990:1266:6aff:fe1f:50b7
      hwaddr: 10:66:6a:1f:50:b7
      mtu: 1500
      roles:
      - management
      - cluster
      routes:
      - to: default
        via: 10.234.136.1
      speed: "-1"
      state: routable
      stats:
        rx_bytes: 60644
        rx_errors: 0
        tx_bytes: 113337
        tx_errors: 0
      type: interface
```

Then, edit the interface and/or bond to add the appropriate role by running `incus admin os system edit network`:

```
config:
  interfaces:
  - addresses:
    - dhcp4
    - slaac
    hwaddr: 10:66:6a:1f:50:b7
    lldp: false
    name: enp5s0
    required_for_online: "no"
    roles:
    - instances
```

After the configuration change is applied, a new unmanaged bridge will appear:

```
gibmat@futurfusion:~$ incus network list
+----------+--------+---------+----------------+---------------------------+----------------------------+---------+---------+
|   NAME   |  TYPE  | MANAGED |      IPV4      |           IPV6            |        DESCRIPTION         | USED BY |  STATE  |
+----------+--------+---------+----------------+---------------------------+----------------------------+---------+---------+
| enp5s0   | bridge | NO      |                |                           |                            | 0       |         |
+----------+--------+---------+----------------+---------------------------+----------------------------+---------+---------+
| incusbr0 | bridge | YES     | 10.89.179.1/24 | fd42:e1a1:408f:710a::1/64 | Local network bridge (NAT) | 1       | CREATED |
+----------+--------+---------+----------------+---------------------------+----------------------------+---------+---------+
```

Now, you can configure an instance to directly connect to the host's physical network:

```
gibmat@futurfusion:~$ incus launch images:debian/13 debian-nat
Launching debian-nat
gibmat@futurfusion:~$ incus launch images:debian/13 debian-direct --network enp5s0
Launching debian-direct
gibmat@futurfusion:~$ incus list
+---------------+---------+-----------------------+------------------------------------------------+-----------+-----------+
|     NAME      |  STATE  |         IPV4          |                      IPV6                      |   TYPE    | SNAPSHOTS |
+---------------+---------+-----------------------+------------------------------------------------+-----------+-----------+
| debian-direct | RUNNING | 10.234.136.199 (eth0) | fd42:3cfb:8972:3990:1266:6aff:fe71:14e0 (eth0) | CONTAINER | 0         |
+---------------+---------+-----------------------+------------------------------------------------+-----------+-----------+
| debian-nat    | RUNNING | 10.89.179.217 (eth0)  | fd42:e1a1:408f:710a:1266:6aff:fef6:4995 (eth0) | CONTAINER | 0         |
+---------------+---------+-----------------------+------------------------------------------------+-----------+-----------+
```
