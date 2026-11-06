# Multipath

The Multipath service allows configuring multipath storage devices.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_multipath.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the Multipath service.

* `wwns`: An array of storage device {abbr}`WWN (World Wide Name)`s to configure for multipath. These should be lowercase hexadecimal strings with no colon separators and enclosed in single quotes, for example `'0x5abcdef123456001'`. See the available disk `WWN`s in the output of `incus admin os system storage show`. Cannot be combined with `automatic_transports`.

* `automatic_transports`: An array of transports on which every storage device is automatically configured for multipath, without having to list its {abbr}`WWN (World Wide Name)`. Supported values are `fc` for Fibre Channel and `iscsi`. Cannot be combined with `wwns`.

## Automatic configuration

When `automatic_transports` is set, IncusOS configures every storage device reached over the listed transports for multipath as soon as it is detected, including devices that only have a single path. Devices and paths that show up later are picked up without any configuration change.

```yaml
config:
  enabled: true
  automatic_transports:
  - fc
  - iscsi
```

The resulting multipath device is named after its identifier and is available as `/dev/mapper/<identifier>`, which is the same on every server that can reach the device. The detected devices and their paths are reported in the service state.

```{warning}
In this mode, only devices on the listed transports are configured for multipath. Devices on any other transport are ignored, including one that IncusOS may have booted from, which is why `wwns` can't be set at the same time. Systems that need multipath on other devices, such as SAS drives with two paths, must keep listing them through `wwns` instead.
```

```{note}
A device that is already in use through one of its paths when this mode is enabled is left untouched. It only gets configured for multipath once it is no longer in use, which in practice means on the next boot.
```
