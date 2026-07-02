# Installation seed
IncusOS depends on an "install seed" to automate the installation process.

That seed is normally automatically generated when [getting an image](../getting-started/download.md).

For more advanced use, it's possible to provide your own seed data using the information below.

## Format and location
The install seed is a simple tar archive consisting of one or more JSON or YAML
configuration files. The tar file is written directly to the start of the second
partition of the install image. At runtime, IncusOS will attempt to read the
install seed from the second partition and use any data present during the
install process.

Alternatively, a user-provided seed partition may be provided independent of
the install image. The partition label must be `SEED_DATA` on either a USB
drive formatted as FAT or an ISO image. Rather than reading a tar archive,
the install logic will attempt to directly read the JSON or YAML configuration
files from the mounted file system. Upon completion of the install, it is up
to the user to disconnect their seed device from the machine, otherwise Incus
OS will become confused when it starts up and detects that seed data is still
present. (The install process wipes the seed data tar archive from the final
install, but we cannot do this with a user-provided seed.)

## Seed contents
The following configuration files are currently recognized:

### `install.{json,yml,yaml}`
The presence of this file, even if empty, will trigger IncusOS to start the
installation process.

The structure is defined in [`api/seed/install.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/install.go):

- `force_install`: If true, will install to target device even if partitions
  already exist. WARNING: THIS CAN CAUSE DATA LOSS!

- `force_install_confirmation`: An optional value used when `force_install` is true
  and IncusOS is already installed on the target device. To guard against accidental
  data loss, the first six characters of the currently installed IncusOS system must
  be provided. (The installer will provide this as part of its error message if this
  field is unset in the install seed.)

- `force_reboot`: If true, reboot after install without waiting for removal of
  install media.

- `security`: An optional struct to enable IncusOS to run in a degraded security
  state. WARNING: This shouldn't be set unless you know exactly what you are doing
  and understand the security implications.

- `target`: An optional struct used to determine the install target device.
  If not specified, IncusOS will expect a single unused drive to be present
  during install. Supported selectors include:
   - `bus`: Bus type of the disk, for example "NVME", "SCSI", or "USB" (case insensitive)
   - `id`: Disk ID as listed in `/dev/disk/by-id/`, will be used in a case-sensitive sub-string match
   - `max_size`: Maximum size of the install disk, such as 1TiB
   - `min_size`: Minimum size of the install disk, such as 100GiB
   - `sort_order`: Optional, either "largest" or "smallest"; if defined, sort potential targets by their capacity and pick the first one

### `applications.{json,yml,yaml}`
This file defines what applications should be installed after IncusOS is up and
running.

The structure is defined in [`api/seed/applications.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/applications.go):

- `applications`: Holds an array of applications to install. Currently supported applications
  can be viewed [here](./applications.md). At most one primary application can be specified,
  and if no primary application is specified the `incus` application will be automatically appended
  to any other provided applications.

### `incus.{json,yml,yaml}`
This file provides preseed information for Incus.

The structure is defined in [`api/seed/incus.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/incus.go)
and references Incus' [`InitPreseed` API](https://github.com/lxc/incus/blob/main/shared/api/init.go):

- `apply_defaults`: If true, automatically apply a set of reasonable defaults
  when installing Incus.

- `preseed`: Additional preseed information to be passed to Incus during
  install.

### `kernel.{json,yml,yaml}`
This file defines kernel configuration options that may need to be set before the
installation process begins or the IncusOS API is available to fully configure
available kernel options.

The structure is defined in [`api/seed/kernel.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/kernel.go):

- `console`: An array of console devices that should be used by the IncusOS
  terminal user interface. Optionally, a baud rate may be specified to configure
  the speed of that specific console device.

### `network.{json,yml,yaml}`
This file defines what network configuration should be applied when IncusOS
boots. If not specified, IncusOS will attempt automatic {abbr}`DHCP (Dynamic Host Configuration Protocol)`/{abbr}`SLAAC (Stateless Address Configuration)`
configuration on each network interface.

The structure used is the [network API struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_network.go):

- `dns`: Optional, configure specific DNS servers as well as the system's
  host name and domain name.

- `time`: Optional, configure specific NTP servers and the server's timezone.

- `proxy`: Optional, configure the system-wide proxy.

- `interfaces`, `bonds`, `vlans`, and `wireguard`: Define one ore more interfaces,
  bonds, VLANS or WireGuard tunnels for use by IncusOS.

### `migration-manager.{json,yml,yaml}`
This file provides preseed information for Migration Manager.

The structure is defined in [`api/seed/migration_manager.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/migration_manager.go)
and references Migration Manager's [`system` API](https://github.com/FuturFusion/migration-manager/blob/main/shared/api/system.go):

- `apply_defaults`: If true, automatically apply a set of reasonable defaults
  when installing Migration Manager.

- `trusted_client_certificates`: A list of one or more PEM-encoded client TLS
  certificates that should be trusted by Migration Manger.

- `preseed`: Additional preseed information to be passed to Migration Manager during
  install.

### `operations-center.{json,yml,yaml}`
This file provides preseed information for Operations Center.

The structure is defined in [`api/seed/operations_center.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/operations_center.go)
and references Operations Center's [`system` API](https://github.com/FuturFusion/operations-center/blob/main/shared/api/system/system.go):

- `apply_defaults`: If true, automatically apply a set of reasonable defaults
  when installing Operations Center.

- `trusted_client_certificates`: A list of one or more PEM-encoded client TLS
  certificates that should be trusted by Operations Center.

- `preseed`: Additional preseed information to be passed to Operations Center during
  install.

### `provider.{json,yml,yaml}`
This file provides preseed information to configure a given provider, which is used
to fetch IncusOS updates and applications.

The structure used is the [provider API struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_provider.go):

- `name`: The provider name; must be one of "images", "operations-center", or "debug".

- `config`: A map that defines provider-specific configuration.

```{note}
The "debug" provider is not intended for general use, and should only be used to
support work developing IncusOS.
```

### `security.{json,yml,yaml}`
This file provides security configuration for the system.

The structure used is the [security API struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_security.go):

- `custom_ca_certs`: An array of PEM-encoded CA certificates that should be
  added to the IncusOS trust store.

```{note}
It is not possible to set encryption recovery key(s) via the security seed. This is
because the seed must be stored in plain text, which would allow trivial access to
anyone trying to compromise the encrypted IncusOS root partition.
```

### `update.{json,yml,yaml}`
This file provides update configuration for the system.

The structure used is the [update API struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_update.go):

- `auto_reboot`: If true, automatically reboot the system after applying an OS update.

- `channel`: The channel to use when checking for updates. Defaults to "stable"; depending
  on the provider other values such as "testing" may be valid.

- `check_frequency`: How often IncusOS should check for updates. Defaults to six hours, and
  accepts values like "6h", "1d", "3h 30m"; the special value "never" disables automatic
  update checks.

- `maintenance_windows`: Optional, defining one or more maintenance windows will limit when
  IncusOS will check for and apply updates.
