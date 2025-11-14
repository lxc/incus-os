# IncusOS
IncusOS is an immutable OS solely designed around safely and reliably
running Incus. It uses modern security features like UEFI Secure Boot
and TPM to provide a safe boot experience and seamless full disk
encryption.

Updates are applied atomically using an A/B scheme allowing for an easy
revert in case of problems.

The system itself is completely locked down with no local or remote
shell, only an authenticated REST API to access Incus and manage the OS
through it.

IncusOS is ideal for anyone who's focused on building and running
infrastructure on top of Incus and wants the underlying infrastructure
to be reliable and easy to update.

All IncusOS servers are guaranteed to be running bit for bit the same
software, eliminating any deployment variance and making it trivial to
scale or re-deploy even large number of servers.

## Core features

Main design features:

- Boot safety (UEFI Secure Boot and TPM 2.0 measurements)
- Full disk encryption (TPM backed LUKS and ZFS encryption)
- Immutable (A/B partition scheme, all OS partitions read-only and signed)
- Locked down (API only management)
- Designed for modern Intel/AMD or ARM systems

Storage features:

- Automatic local ZFS pool
- Support for complex ZFS pool creation on additional disks
- Fiber Channel & Multipath support
- NVME-over-TCP support
- iSCSI support
- Clustered LVM support (on top of Fiber Channel, NVME-over-TCP or iSCSI)
- Ceph and Linstor support for software defined storage

Network features:

- Automatic VLAN-aware bridging making it easy to attach instances to any interface
- Link aggregation support (both passive and negotiated)
- LLDP support
- Support for enterprise proxy servers (including Kerberos authentication)
- Robust NTP support
- Remote logging support through syslog (UDP, TCP, TLS)
- OVS/OVN support for software defined networking
- Native support for Tailscale (Netbird coming soon)

Management features:

- Central management through Operations Center
- Backup/Restore of both the main OS configuration and individual application data
- Factory reset of either the whole OS or individual applications
- Flexible update management

## Technical details
IncusOS is built on top of Debian 13 with our own Incus and kernel builds.

In addition to running Incus itself, IncusOS can also be used as the
underlying OS to run Operations Center and Migration Manager, allowing
for an easy migration from a VMware or similar environment over to
Incus.

We make extensive use of systemd's modern OS features to build our
images, handle updates and take care of things like first boot
partitioning and TPM backed disk encryption.

## Updates and release cadence
We currently maintain two update channels for IncusOS:

- stable
- testing

All installations default to the `stable` channel which typically sees
at least one weekly update to pick up the latest stable bug fix release
of the Linux kernel as well as any relevant security issues.

The `testing` channel sees much more frequent builds, typically once a
day.

IncusOS systems default to checking for updates every 6 hours and will
automatically update Incus itself with a very short API downtime (no
impact to running instances) and will stage any OS update to be booted
upon reboot.

Configuration options are available to change the update frequency or
disable automatic updates altogether as well as specifying scheduled
downtime periods to apply the application updates.

```{toctree}
:hidden:
:titlesonly:

self
Getting started </getting-started>
Reference </reference>
Contributing </contributing>
Support </support>
```
