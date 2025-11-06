# iSCSI

The iSCSI service allows connecting a remote iSCSI storage device over TCP.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_iscsi.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the iSCSI service.

* `targets`: An array of iSCSI targets, each of which consists of an address, port, and iSCSI target.
