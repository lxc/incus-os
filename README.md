# Introduction
Incus OS is a minimal immutable OS image dedicated to running [Incus](https://linuxcontainers.org/incus).
It's based on [Debian](https://www.debian.org) Testing and built using [mkosi](https://github.com/systemd/mkosi).

This aims at provided a very fast, safe and reliable way to run an Incus server.

# Security features
Incus OS is designed to run on systems using UEFI with Secure Boot enabled.
On first boot, it will automatically add the relevant Secure Boot key
(requires the system be in setup mode).

This ensures that only our signed image can be booted on the system.
The image then uses dm-verity to validate every bit that's read from disk.

All throughout boot, artifacts get measured through the TPM with the TPM
state used to manage disk encryption.

This effectively ensures that the system can only boot valid Incus OS
images, that nothing can be altered on the system and that any
re-configuration of the system requires the use of a recovery key to
access the persistent storage.

When updating, Incus OS uses an A/B update mechanism to reboot onto the
newer version while keeping the previous version available should a
revert be needed.

# Status
Incus OS is still in early development, the instructions below are there
to help try it out, mostly for testing purposes as new features get
added.

# Testing
Currently all development and testing of Incus OS is done through Incus VMs.
The instructions below assume a functional Incus environment with VM support.

## Using the Github releases
Two scripts are available to test Incus OS using the publicly published releases.

Creating a new Incus OS VM can be done with:
```bash
    ./scripts/spawn-image VERSION NAME
```

This will retrieve the relevant image from Github and create a VM using it.
It will also automatically load the relevant packages (`incus` and `debug`).

To update it to a newer version, you can do:
```bash
    ./scripts/update-image VERSION NAME
```

This will retrieve the update files and apply the update.

## Setup Debian 13 build environment ##
```bash
sudo apt update -y && sudo apt install git mkosi build-essential wget
```

## Install GO ##
```bash
mkdir -p ~/Downloads && pushd ~/Downloads && wget -c https://go.dev/dl/go1.24.2.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local/ -xzf go1.24.2.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/.local/go
popd
```

## By building your own images
Building your own images require the current version of `mkosi`.

To build an image, run:
```bash
    make
```

To load that image as a VM, run:
```bash
    make test
```

To load the packages, run:
```bash
    make test-extensions
```

To test an update, build a new image and update to it with:
```bash
    make
    make update
```
