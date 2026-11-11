# Incus

The Incus application includes Incus as packaged by [Zabbly](https://github.com/zabbly/incus). It includes everything needed to run containers, OCI images, and virtual machines.

```{warning}
At least one trusted client certificate must be provided in the Incus preseed, otherwise it will be impossible to authenticate to any API endpoint or the web UI post-install.
```

There are currently two "flavors" of the Incus application which can be installed:

* The current stable (aka "monthly" or "feature") release is available as the `incus` application and receives new features and bug fixes on a regular monthly schedule. This is the default version of Incus selected by IncusOS if not otherwise configured.

* The {abbr}`LTS (Long Term Support)` 7.0 series of releases is available as the `incus-lts-7.0` application. The LTS series of Incus is recommended for enterprise environments and will receive only bug and security fixes for the duration of its five year support cycle.

```{note}
It is possible to switch between the stable and LTS Incus release tracks under certain conditions:

* It is always possible to switch from a LTS release to the current stable release.

* It is possible to switch from a stable release to the LTS series, but _only_ from versions of Incus less than or equal to the first LTS release of that LTS series. For example, it's possible to switch both Incus versions 6.23.0 and 7.0.0 to the 7.0 LTS series, but it is NOT possible to switch Incus version 7.1.0 to the 7.0 LTS series.

These limitations are due to how Incus tracks internal database schema changes. It's always possible to move forward, but rolling back to a prior schema version is not supported.

IncusOS can switch the installed release of Incus, if supported, by running `incus admin os application add <desired incus application>`.
```

## Default configuration

If the Incus seed field `apply_defaults` is `true`, the Incus application will perform the following initialization steps:

* Create a default ZFS-backed storage pool "local" for use by Incus. This storage pool will use all remaining free space on the main system drive.

* Create a local network bridge `incusbr0`.

* Set the list of provided trusted client certificates.

* Listen on port 8443 on all network interfaces.

## Install seed details

Important seed fields include:

* `apply_defaults`: If `true`, apply a reasonable set of defaults for configuring Incus.

* `preseed`: A struct referencing Incus' `InitPreseed` configuration options. For details, please review Incus' [API](https://github.com/lxc/incus/blob/main/shared/api/init.go).

## Configuration options

Configuration fields are defined in the [`ApplicationIncusConfig` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/application_incus.go).

On top of the [shared application options](shared-api.md), the following configuration options can be set:

* `lxcfs`: Change the behavior of LXCFS, which provides containers with a view of the system resources matching their limits. Incus is restarted when changing these options.
   * `cpu_shares`: If `true`, also take the CPU allowance of a container into account when reporting the CPUs available to it.
   * `load_average`: If `true`, report a per-container load average rather than the one of the whole system.

## Additional features

Two additional applications exist which extend the main Incus application:

* `incus-ceph`: Adds [Ceph](../services/ceph.md) client support
* `incus-linstor`: Adds [Linstor](../services/linstor.md) satellite support

## Actions

The Incus application supports the following actions to deploy and manage a Ceph cluster on top of Incus. See the [Ceph cluster tutorial](../../tutorials/storage-create-ceph-cluster.md) for their parameters:

* `ceph-initialize`: Creates the Ceph control plane.
* `ceph-add-drive`: Adds a drive to the Ceph cluster.
* `ceph-remove-drive`: Removes a drive from the Ceph cluster.
* `ceph-refresh-images`: Updates the deployed version of Ceph.

You can run actions using:

```
incus admin os application action incus -d '{"action":"ceph-initialize","config":{"control_servers":"server01,server02,server03"}}'
```

### Changing cluster addresses

The `cluster-set-addresses` action changes the address that Incus cluster members use to reach each other. Its configuration maps cluster member names to their new address in `host:port` format. Members which aren't listed keep their current address.

The action only updates the Incus database of the server it is run on, and can only do so while Incus is stopped. It therefore has to be run on every cluster member, with the same configuration, while Incus is stopped on all of them:

1. Stop all instances, for example by evacuating every cluster member with the `stop` action.
1. Make sure the [fallback listener](../system/fallback-listener.md) is active on every server, for example by setting its `persistent` option, and that it can be reached.
1. Stop the Incus application on every server.
1. Through the fallback listener, apply any needed [network configuration](../system/network.md) change and run the `cluster-set-addresses` action on every server.
1. Start the Incus application on every server.
1. Confirm that all cluster members are back online, then restore the evacuated members.

For example, when moving a three server cluster to a new subnet:

```
incus admin os application stop incus
incus admin os application action incus -d '{"action":"cluster-set-addresses","config":{"server01":"10.0.0.1:8443","server02":"10.0.0.2:8443","server03":"10.0.0.3:8443"}}'
incus admin os application start incus
```

The new addresses are applied to the cluster database when Incus starts. A server whose `core.https_address` was set to its old cluster address is updated to listen on the new one.

## Debugging

The Incus application supports the following debug actions:

* `get-logs`: Returns a `.tar.gz` of the content of `/run/incus` and `/var/log/incus`

You can run debug actions using:

```
incus admin os application debug incus -d '{"action": "get-logs"}' > logs.tar.gz
```
