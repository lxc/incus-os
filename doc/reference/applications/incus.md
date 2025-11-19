# Incus

The Incus application includes the current Incus feature release as packaged from the [Zabbly stable channel](https://github.com/zabbly/incus). It includes everything needed to run containers, OCI images, and virtual machines.

At least one trusted client certificate must be provided in the Incus preseed, otherwise it will be impossible to authenticate to any API endpoint or the web UI post-install.

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
