# Multipath

The Multipath service allows configuring multipath storage devices.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_multipath.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the Multipath service.

* `wwns`: An array of storage device {abbr}`WWN (World Wide Name)`s to configure for multipath. These should be lowercase hexadecimal strings with no colon separators and are typically prefixed with a `3`. The correct format is seen in the output of `incus admin os system storage show` under the `id` field, for example `/dev/disk/by-id/scsi-<wwn>`.
