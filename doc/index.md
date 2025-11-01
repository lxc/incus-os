# Incus OS documentation
Incus OS is still in early alpha development. The instructions below are
there to help try it out, mostly for testing purposes as new features get
added.

## System Requirements

Incus OS is designed to provide an extremely secure environment in which to
run Incus. It requires a lot of modern system features and will not function
properly on older unsupported systems.

The installed system is only reachable through Incus; there is no local shell
access or remote access through SSH. You will need to provide a trusted Incus
client certificate when preparing your install media so you can connect to
the system post install.

Minimum system requirements:

- Modern x86_64 or arm64 system (5 years old at most)
- Support for UEFI with Secure Boot
- {abbr}`TPM (Trusted Platform Module)` 2.0 security module
- At least 50GiB of storage
- At least one wired network port

## Installation
ISO and raw images are distributed via the [Linux Containers {abbr}`CDN (Content Delivery Network)`](https://images.linuxcontainers.org/os/).

There are two more user-friendly methods to get an Incus OS install image: a
web-based customization tool and a command line flasher tool.

Incus OS doesn't feature a traditional installer, and relies on an [install seed](install-seed.md)
to provide configuration details and defaults during install. This install
seed can be manually crafted, or you can use either of the two utilities
described below to automate the process.

After configuring your Incus OS image, you can then boot and Incus OS will
automatically install itself. Upon reboot, Incus OS will automatically start
up and will by default check for updates every six hours.

If the raw image is written to a sufficiently large writable medium (at least
50GiB), such as a USB stick or hard drive, without any install seed Incus OS
will automatically resize on first boot and start running directly from that
media.

### Incus OS customizer

The web-based [Incus OS customizer](https://incusos-customizer.linuxcontainers.org/ui/)
is the most user-friendly way to get an Incus OS install image. The web page
will let you make a few simple selections, then directly download an install
image that's ready for immediate use.

### Flasher tool

A [flasher tool](flasher-tool.md) is provided for more advanced users who need
to perform more customizations of the install seed than the web-based customizer
supports. The flasher can be built by running `go build ./cmd/flasher-tool/`.

## Building locally
You can build Incus OS locally. Only users specifically interested in the
development and testing of new Incus OS features should need to do this.
Building your own images requires a current version of `mkosi`, and should work
on most Linux distributions, with Debian/Ubuntu being the most well-tested.

After cloning the repository from GitHub, simply run:

    make

By default the build will produce a raw image in the `mkosi.output/` directory,
suitable for writing to a USB stick. It is also possible to build an ISO
image if you need to boot from a (virtual) CD-ROM device:

    make build-iso

## Testing
Currently all development and testing of Incus OS is done through Incus virtual machines.
These instructions assume a functional Incus environment with virtual machine support.

### Local builds
To test a locally built raw image in an Incus virtual machine, run:

    make test

After Incus OS has completed its installation and is running in the virtual machine, to load
applications run:

    make test-applications

To test the update process, build a new image and update to it with:

    make
    make test-update

### Using official releases
A script is available to test Incus OS using the public releases. It depends on
the flasher tool being available to automate the download of the latest Incus OS
release.

Creating a new Incus OS virtual machine can be done with:

    ./scripts/spawn-image NAME

This will retrieve the most recent image from the Linux Containers CDN and
create a virtual machine. It will also automatically load the relevant packages (currently
just `incus`).

The virtual machine will automatically check for updates every 6 hours with the OS updates
applying on reboot.

```{toctree}
:hidden:
:titlesonly:

self
basic-install-steps
flasher-tool
secure-boot
system-recovery
install-seed
rest-api
```
