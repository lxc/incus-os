# NVMe

The NVMe service allows connecting a remote NVMe storage device over fibre channel or TCP.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_nvme.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the NVMe service.

* `targets`: An array of NVMe targets, each of which consists of an address, port, and transport type.
