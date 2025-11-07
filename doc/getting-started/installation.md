# Installing IncusOS
IncusOS is designed to run on modern physical hardware as that's the
optimal environment to run an Incus server.

But we also support having it run inside of a virtual machine, making it
easier to evaluate or debug. In general, any physical or virtual
environment which matches our [hardware requirements](requirements.md)
should do fine. That said we recommend using generic storage and network
adapters whenever possible, with NVMe, VirtIO or Intel virtual devices
usually being preferred.

```{toctree}
:maxdepth: 1

Installing on hardware </getting-started/installation/physical>
Installing on Incus </getting-started/installation/virtual-incus>
Installing on Proxmox </getting-started/installation/virtual-proxmox>
Installing on VirtualBox </getting-started/installation/virtual-virtualbox>
Installing on VMware </getting-started/installation/virtual-vmware>
```
