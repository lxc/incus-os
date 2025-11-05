![Daily API tests](https://github.com/lxc/incus-os/actions/workflows/daily.yml/badge.svg)

# Introduction
IncusOS is a minimal immutable OS image dedicated to running [Incus](https://linuxcontainers.org/incus).
It's based on [Debian](https://www.debian.org) trixie and built using [mkosi](https://github.com/systemd/mkosi).
IncusOS can be installed on modern amd64 (x86_64) and arm64 systems.

This aims at providing a very fast, safe and reliable way to run an Incus server.

# Security features
IncusOS is designed to run on systems using UEFI with Secure Boot enabled.
On first boot, it will automatically add the relevant Secure Boot keys
(requires the system be in setup mode).

This ensures that only our signed image can be booted on the system.
The image then uses dm-verity to validate every bit that's read from disk.

All throughout boot, artifacts get measured through the TPM with the TPM
state used to manage disk encryption.

This effectively ensures that the system can only boot valid IncusOS
images, that nothing can be altered on the system and that any
re-configuration of the system requires the use of a recovery key to
access the persistent storage.

When updating, IncusOS uses an A/B update mechanism to reboot onto the
newer version while keeping the previous version available should a
revert be needed.

# Status
IncusOS is still in early alpha development, which means it comes with some
important caveats:

  * There can and will be breaking changes, which may ultimately require a
  fresh reinstall. Therefore, DO NOT use IncusOS with any kind of important
  data.
  
  * Currently all development and testing of IncusOS is done through Incus
  VMs. While it should be possible to run IncusOS on physical hardware or
  other virtualization solutions (ie, Proxmox), support will be limited.
  
  * IncusOS is intentionally opinionated and requires modern hardware to
  enable its various security features. IncusOS will never be installable
  on systems without UEFI Secure Boot and a TPM.

# Documentation
More detailed documentation is available in the `doc/` directory, including
a [brief example](doc/basic-install-steps.md) of how to configure and then
connect to Incus post-install.
