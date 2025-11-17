# Applying VLAN tagging to physical networks

IncusOS can assign automatic VLAN tagging for one or more VLANs to any configured interface or bond.

This tutorial will assume the network interface is named `enp5s0` and the VLAN ID we want to set is `1234`.

To assign the VLAN tagging, we need to make sure the interface is assigned both the `instances` [role](../reference/system/network.md) and that the `vlan_tags` property consists of a list of desired VLAN ID(s). This can be done by running `incus admin os system edit network` and edit the configuration like follows:

```
config:
  interfaces:
  - addresses:
    - dhcp4
    - slaac
    hwaddr: 10:66:6a:d2:32:18
    lldp: false
    name: enp5s0
    required_for_online: "no"
    roles:
    - instances
    vlan_tags:
    - 1234
```

After the configuration change is applied, a new unmanaged bridge will appear:

```
gibmat@futurfusion:~$ incus network list
+----------+--------+---------+-----------------+---------------------------+----------------------------+---------+---------+
|   NAME   |  TYPE  | MANAGED |      IPV4       |           IPV6            |        DESCRIPTION         | USED BY |  STATE  |
+----------+--------+---------+-----------------+---------------------------+----------------------------+---------+---------+
| enp5s0   | bridge | NO      |                 |                           |                            | 0       |         |
+----------+--------+---------+-----------------+---------------------------+----------------------------+---------+---------+
| incusbr0 | bridge | YES     | 10.148.244.1/24 | fd42:15d0:aec3:c78d::1/64 | Local network bridge (NAT) | 1       | CREATED |
+----------+--------+---------+-----------------+---------------------------+----------------------------+---------+---------+

```

Create a managed network for VLAN `1234`:

```
gibmat@futurfusion:~$ incus network create enp5s0.1234 parent=enp5s0 vlan=1234 --type=physical
Network enp5s0.1234 created
gibmat@futurfusion:~$ incus network list
+-------------+----------+---------+-----------------+---------------------------+----------------------------+---------+---------+
|    NAME     |   TYPE   | MANAGED |      IPV4       |           IPV6            |        DESCRIPTION         | USED BY |  STATE  |
+-------------+----------+---------+-----------------+---------------------------+----------------------------+---------+---------+
| enp5s0      | bridge   | NO      |                 |                           |                            | 1       |         |
+-------------+----------+---------+-----------------+---------------------------+----------------------------+---------+---------+
| enp5s0.1234 | physical | YES     |                 |                           |                            | 0       | CREATED |
+-------------+----------+---------+-----------------+---------------------------+----------------------------+---------+---------+
| incusbr0    | bridge   | YES     | 10.148.244.1/24 | fd42:15d0:aec3:c78d::1/64 | Local network bridge (NAT) | 1       | CREATED |
+-------------+----------+---------+-----------------+---------------------------+----------------------------+---------+---------+
```

Now, you can configure an instance to use VLAN `1234`:

```
gibmat@futurfusion:~$ incus launch images:debian/13 debian --network enp5s0.1234
Launching debian
gibmat@futurfusion:~$ incus list
+--------+---------+-----------------------+------------------------------------------------+-----------+-----------+
|  NAME  |  STATE  |         IPV4          |                      IPV6                      |   TYPE    | SNAPSHOTS |
+--------+---------+-----------------------+------------------------------------------------+-----------+-----------+
| debian | RUNNING | 10.234.136.199 (eth0) | fd42:3cfb:8972:3990:1266:6aff:fe71:14e0 (eth0) | CONTAINER | 0         |
+--------+---------+-----------------------+------------------------------------------------+-----------+-----------+
```

You can also make this network the default for all instances:

```
gibmat@futurfusion:~$ incus profile device set default eth0 network=enp5s0.1234
```
