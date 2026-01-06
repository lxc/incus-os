# Kernel

IncusOS exposes a limited set of configuration knobs for adjusting kernel-level settings. A system reboot may be required for changes to fully take effect.

## Configuration options

Configuration fields are defined in the [`SystemKernelConfig` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_kernel.go).

The following configuration options can be set:

* `blacklist_modules`: A list of one or more kernel modules to blacklist. Typically useful when passing through PCI devices to virtual machines.

* `network`: Change `sysctl` values that impact the system's network configuration.
   * `buffer_size`: Optional; configure the maximum buffer size used when setting the `net.ipv4.tcp_rmem`, `net.ipv4.tcp_wmem`, `net.core.rmem_max`, and `net.core.wmem_max` `sysctl` fields.

   * `queuing_discipline`: Optional; configure the value of the `net.core.default_qdisc` `sysctl` field.

   * `tcp_congestion_algorithm`: Optional; configure the TCP congestion algorithm used by the system, defaults to `bbr`.

* `pci`: Change PCI device configuration.
   * `passthrough`: Configure one or more PCI devices for pass-through to a virtual machine:
      * `vendor_id`: The PCI vendor ID
      * `product_id`: The PCI product ID
      * `pci_address`: Optional; if specified the system will attempt to unbind the given PCI device from its existing driver and configure it for passing though to a virtual machine without requiring a reboot.
