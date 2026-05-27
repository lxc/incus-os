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

IncusOS can switch the installed release of Incus, if supported, by running `incus admin os application add -d '{"name":"<desired incus application>"}'`.
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

## Additional features

Two additional applications exist which extend the main Incus application:

* `incus-ceph`: Adds [Ceph](../services/ceph.md) client support
* `incus-linstor`: Adds [Linstor](../services/linstor.md) satellite support

## Debugging

The Incus application supports the following debug actions:

* `get-logs`: Returns a `.tar.gz` of the content of `/run/incus` and `/var/log/incus`

You can run debug actions using:

```
incus admin os application debug incus -d '{"action": "get-logs"}' > logs.tar.gz
```
