# Kernel

IncusOS exposes a limited set of configuration knobs for adjusting kernel-level settings. A system reboot may be required for changes to fully take effect.

## Configuration options

Configuration fields are defined in the [`SystemKernelConfig` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_kernel.go).

The following configuration options can be set:

* `blacklist_modules`: A list of one or more kernel modules to blacklist. Typically useful when passing through PCI devices to virtual machines.

* `console`: A list of additional console devices used by the IncusOS terminal user interface. Changes apply on the next boot.
   * `device`: The path of the console device, such as `/dev/ttyS1`.
   * `baud_rate`: Optional; the speed to configure on that console device.

* `cpu`: Change CPU-specific settings.
   * `scaling_governor`: Optional; the CPU frequency scaling governor to use, must be one of the governors supported by the kernel.
   * `disable_thermal_protection`: Optional; when true, the kernel stops acting on thermal zone trip points and thermal management is left to the firmware. Useful on boards such as the Raspberry Pi 4 where the firmware already throttles the processor but the critical trip point it exposes to the kernel triggers a shutdown.

* `memory`: Change `sysctl` values or other settings that impact the system's memory configuration
   * `persistent_hugepages`: Optional; number of persistent hugepages to allocate.
   * `zram_swap_size`: Optional; a human-readable string such as "4GiB" that defines the size of the zram-backed swap device to create. A value of "0" will disable any existing zram-backed swap that may exist. IncusOS will attempt to immediately apply this change without requiring a reboot.

* `network`: Change `sysctl` values that impact the system's network configuration.
   * `buffer_size`: Optional; configure the maximum buffer size used when setting the `net.ipv4.tcp_rmem`, `net.ipv4.tcp_wmem`, `net.core.rmem_max`, and `net.core.wmem_max` `sysctl` fields.

   * `netdev_max_backlog`: Optional; configure the value of the `net.core.netdev_max_backlog` `sysctl` field, defaults to `1000`.

   * `queuing_discipline`: Optional; configure the value of the `net.core.default_qdisc` `sysctl` field.

   * `tcp_congestion_algorithm`: Optional; configure the TCP congestion algorithm used by the system, defaults to `bbr`.

   * `tcp_mtu_probing`: Optional; if `true`, enable TCP MTU probing through the `net.ipv4.tcp_mtu_probing` `sysctl` field.

* `pci`: Change PCI device configuration.
   * `passthrough`: Configure one or more PCI devices for pass-through to a virtual machine:
      * `vendor_id`: The PCI vendor ID
      * `product_id`: The PCI product ID
      * `pci_address`: Optional; if specified the system will attempt to unbind the given PCI device from its existing driver and configure it for passing though to a virtual machine without requiring a reboot.
   * `sriov`: Configure the number of SR-IOV virtual functions to create on one or more PCI devices. IncusOS will attempt to immediately apply this change without requiring a reboot.
      * `pci_address`: The PCI address of the device.
      * `vf_count`: The number of virtual functions to create, which can't exceed the maximum supported by the device.

* `zfs`: Change ZFS-specific settings.
   * `arc_max_size`: Optional; a human-readable string such as "8GiB" that defines the maximum size of the ZFS ARC. When unset or "0", the ZFS default is used. IncusOS will attempt to immediately apply this change without requiring a reboot.

## State

The following state is reported alongside the configuration:

* `clock`: The kernel timekeeping clock source.
   * `source`: The clock source currently in use, for example `tsc` or `kvm-clock`.
   * `available_sources`: The clock sources the kernel can switch to.

* `memory`: Memory-related state.
   * `zram_swap`: Statistics about the zram-backed swap device, if one is configured. All sizes are in bytes.
