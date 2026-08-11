# NVMe

The NVMe service allows connecting a remote NVMe storage device over fibre channel or TCP.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_nvme.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the NVMe service.

* `targets`: An array of NVMe targets, each of which consists of:

   * `transport`: The transport type, either `tcp` or `fc`.

   * `address`: With `tcp`, the IP address of the target. With `fc`, the World Wide Names of the remote Fibre Channel port in the `nn-0x<WWNN>:pn-0x<WWPN>` format.

   * `port`: With `tcp`, the port number of the target. Unused with `fc`.

   * `host_address`: With `fc`, the World Wide Names of the local Fibre Channel port to connect from, using the same format as `address`. If unset, all local Fibre Channel ports are used.

   * `nqn`: If set, directly connect to this subsystem NQN instead of relying on the target's discovery controller. This is needed for storage arrays that address volumes by name and don't expose them through their discovery log.
