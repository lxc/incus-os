# {abbr}`USBIP (USB over IP)`

The [{abbr}`USBIP (USB over IP)`](https://usbip.sourceforge.net/) service provides access to remote USB devices over IP.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_usbip.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the USBIP service.

* `targets`: An array of USBIP targets, each of which consists of:

   * `address`: The address of the remote USBIP server.

   * `bus_id`: The bus ID of the USB device on the remote server.
