# Multipath

The Multipath service allows configuring multipath storage devices.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_multipath.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the Multipath service.

* `wwns`: An array of {abbr}`WWN (World Wide Name)`s to configure for multipath.
