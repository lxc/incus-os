# {abbr}`USBIP (USB over IP)`

The [{abbr}`USBIP (USB over IP)`](https://usbip.sourceforge.net/) service provides access to remote USB devices over IP.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_usbip.go).

The following configuration options can be set:

* `targets`: An array of USBIP targets, each of which consists of an address and bus ID.
