# Installing IncusOS
IncusOS is designed to run on modern physical hardware as that's the
optimal environment to run an Incus server.

But we also support having it run inside of a virtual machine, making it
easier to evaluate or debug. In general, any physical or virtual
environment which matches our [hardware requirements](requirements.md)
should do fine. That said we recommend using generic storage and network
adapters whenever possible, with NVMe, VirtIO or Intel virtual devices
usually being preferred.

```{note}
For virtual machines, storage drives should be configured to use the `VirtIO-scsi` driver. Using `VirtIO-blk` does not work as the resulting drives will not appear to IncusOS in the same way as physical drives do.
```

## Unsupported platforms
So far we're aware that IncusOS cannot be installed on top of Microsoft
Hyper-V due to that virtualization platform not supporting custom Secure
Boot keys.

```{toctree}
:maxdepth: 1

Installing on hardware </getting-started/installation/physical>
Installing on Incus </getting-started/installation/virtual-incus>
Installing on libvirt </getting-started/installation/virtual-libvirt>
Installing on Proxmox </getting-started/installation/virtual-proxmox>
Installing on VirtualBox </getting-started/installation/virtual-virtualbox>
Installing on VMware </getting-started/installation/virtual-vmware>
```
