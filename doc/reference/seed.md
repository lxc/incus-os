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

- `force_reboot`: If true, reboot after install without waiting for removal of
  install media.

- `security`: An optional struct to enable IncusOS to run in a degraded security
  state. WARNING: This shouldn't be set unless you know exactly what you are doing
  and understand the security implications.

- `target`: An optional selector used to determine the install target device.
  If not specified, IncusOS will expect a single unused drive to be present
  during install.

### `applications.{json,yml,yaml}`
This file defines what applications should be installed after IncusOS is up and
running.

The structure is defined in [`api/seed/applications.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/applications.go):

- `applications`: Holds an array of applications to install. Currently the
  only supported application are `incus`, `migration-manager`, and `operations-center`.

### `incus.{json,yml,yaml}`
This file provides preseed information for Incus.

The structure is defined in [`api/seed/incus.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/incus.go)
and references Incus' [`InitPreseed` API](https://github.com/lxc/incus/blob/main/shared/api/init.go):

- `apply_defaults`: If true, automatically apply a set of reasonable defaults
  when installing Incus.

- `preseed`: Additional preseed information to be passed to Incus during
  install.

### `network.{json,yml,yaml}`
This file defines what network configuration should be applied when IncusOS
boots. If not specified, IncusOS will attempt automatic {abbr}`DHCP (Dynamic Host Configuration Protocol)`/{abbr}`SLAAC (Stateless Address Configuration)`
configuration on each network interface.

The structure used is the [network API struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_network.go).

### `migration-manager.{json,yml,yaml}`
This file provides preseed information for Migration Manager.

The structure is defined in [`api/seed/migration_manager.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/migration_manager.go)
and references Migration Manager's [`system` API](https://github.com/FuturFusion/migration-manager/blob/main/shared/api/system.go).

### `operations-center.{json,yml,yaml}`
This file provides preseed information for Operations Center.

The structure is defined in [`api/seed/operations_center.go`](https://github.com/lxc/incus-os/blob/main/incus-osd/api/seed/operations_center.go)
and references Operations Center's [`system` API](https://github.com/FuturFusion/operations-center/blob/main/shared/api/system.go).

### `provider.{json,yml,yaml}`
This file provides preseed information to configure a given provider, which is used
to fetch IncusOS updates and applications.

The structure used is the [provider API struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_provider.go).
