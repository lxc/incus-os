# Incus OS documentation
Incus OS is still in early alpha development. The instructions below are
there to help try it out, mostly for testing purposes as new features get
added.

## Installation
ISO and raw images are available from the [Incus OS Releases](https://github.com/lxc/incus-os/releases)
page on GitHub.

Before booting the installation image, an [install seed](install-seed.md)
must first be injected. A [flasher tool](flasher-tool.md) is provided to help
automate this process in a user-friendly manner, and can be downloaded from
each GitHub release.

After configuring the Incus OS image with the flasher tool, you can then boot
and Incus OS will automatically install itself. Upon reboot, Incus OS will
then run and will by default check for updates every six hours.

If the raw image is written to a sufficiently large writable medium (at least
50GiB), such as a USB stick or hard drive, without any install seed Incus OS
will automatically resize on first boot and start running directly from that
media.

## Building locally
You can build Incus OS locally. Only users specifically interested in the
development and testing of new Incus OS features should need to do this.
Building your own images requires a current version of `mkosi`, and should work
on most Linux distributions, with Debian/Ubuntu being the most well-tested.

After cloning the repo from GitHub, simply run:

    make

By default the build will produce a raw image in the `mkosi.output/` directory,
suitable for writing to a USB stick. It is also possible to build an iso
image if you need to boot from a (virtual) CDROM device:

    make build-iso

## Testing
Currently all development and testing of Incus OS is done through Incus VMs.
These instructions assume a functional Incus environment with VM support.

### Local builds
To test a locally built raw image in an Incus VM, run:

    make test

After Incus OS has completed its installation and is running in the VM, to load
applications run:

    make test-applications

To test the update process, build a new image and update to it with:

    make
    make test-update

### Using the GitHub releases
A script is available to test Incus OS using the publicly published releases.

Creating a new Incus OS VM can be done with:

    ./scripts/spawn-image VMNAME

This will retrieve the most recent image from Github and create a VM. It will
also automatically load the relevant packages (`incus` and `debug`).

The VM will automatically check for updates every 6 hours with the OS updates
applying on reboot.
