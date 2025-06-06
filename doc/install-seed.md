# Install Seed
Incus OS depends on an "install seed" to automate the installation process. Most
users should use the [flasher tool](flasher-tool.md) which provides a simple way
to configure the install seed without requiring detailed understanding of the
technical details below.

## Format and location
The install seed is a simple tar archive consisting of one or more json/yaml
configuration files. The tar file is written directly to the start of the second
partition of the install image. At runtime, Incus OS will attempt to read the
install seed from the second partition and use any data present during the
install process.

## Seed contents
The following configuration files are currently recognized:

### `install.{json,yml,yaml}`
The presence of this file, even if empty, will trigger Incus OS to start the
installation process.

The structure is defined in [internal/seed/install.go](https://github.com/lxc/incus-os/blob/main/incus-osd/internal/seed/install.go):

  * `ForceInstall`: If true, will install to target device even if partitions
  already exist. WARNING: THIS CAN CAUSE DATA LOSS!
  
  * `ForceReboot`: If true, reboot after install without waiting for removal of
  install media.
  
  * `Target`: An optional selector used to determine the install target device.
  If not specified, Incus OS will expect a single unused drive to be present
  during install.
  
### `applications.{json,yml,yaml}`
This file defines what applications should be installed after Incus OS is up and
running.

The structure is defined in [internal/seed/applications.go](https://github.com/lxc/incus-os/blob/main/incus-osd/internal/seed/applications.go):

  * `Applications`: Holds an array of applications to install. Currently the
  only supported application is "incus".

### `incus.{json,yml,yaml}`
This file provides preseed information for Incus.

The structure is defined in [internal/seed/incus.go](https://github.com/lxc/incus-os/blob/main/incus-osd/internal/seed/incus.go)
and references Incus' [`InitPreseed` API](https://github.com/lxc/incus/blob/main/shared/api/init.go):

  * `ApplyDefaults`: If true, automatically apply a set of reasonable defaults
  when installing Incus.
  
  * `Preseed`: Additional preseed information to be passed to Incus during
  install.

### `network.{json,yml,yaml}`
This file defines what network configuration should be applied when Incus OS
boots. If not specified, Incus OS will attempt automatic DHCP/SLAAC
configuration on each network interface.

The structure used is the [network API struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_network.go).

### `provider.{json,yml,yaml}`
This file provides preseed information to configure a given provider, which is used
to fetch Incus OS updates and applications.

The structure used is the [provider API struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_provider.go).
