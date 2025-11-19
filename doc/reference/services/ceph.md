# Ceph

The [Ceph](https://ceph.io/) service allows connecting a Ceph storage cluster. In addition to Incus, the `incus-ceph` application must be installed to enable this service.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_ceph.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the Ceph service.

* `clusters`: A map of Ceph clusters to connect to.
