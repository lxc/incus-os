# LVM

The LVM service allows configuring of a clustered LVM storage backend.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_lvm.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the LVM service.

* `system_id`: A cluster-unique host identifier.
