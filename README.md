# Introduction
[IncusOS](https://linuxcontainers.org/incus-os) is an immutable OS image dedicated to running [Incus](https://linuxcontainers.org/incus).
It's based on [Debian](https://www.debian.org) 13 and built using [mkosi](https://github.com/systemd/mkosi).

IncusOS can be installed on modern amd64 (x86_64) and arm64 systems.

This aims at providing a very fast, safe and reliable way to run an Incus server.
It's got a strong focus on security, actively relying on UEFI Secure Boot and TPM 2.0 for boot security and disk encryption.

You can read more about how to get started with IncusOS
[here](https://linuxcontainers.org/incus-os/docs/main/getting-started/)
including detailed instructions for physical installation or for running
IncusOS on a variety of virtual machine platforms.

The full documentation for IncusOS can be [found here](https://linuxcontainers.org/incus-os/docs/main).

# Development
This repository includes all the sources used to build the production IncusOS images.

Builds are triggered by pushing a new tag to this repository which kicks
in a full image build, that then gets downloaded and validated by our
publishing server. The image is then made available in the `testing`
channel until it's manually validated and promoted to the `stable`
channel.

The most recent image build logs can be found here: https://github.com/lxc/incus-os/actions/workflows/build.yml  
With the resulting images being published to: https://images.linuxcontainers.org/os/

A daily test is also run, exercising most of the API endpoints and
running tests that would be impractical (too slow) to run for every pull
request.

[![Daily API tests](https://github.com/lxc/incus-os/actions/workflows/daily.yml/badge.svg)](https://github.com/lxc/incus-os/actions/workflows/daily.yml)

# Contributing
This repository is released under the terms of the Apache 2.0 license.

Detailed contribution guidelines can be found in [our documentation](https://linuxcontainers.org/incus-os/docs/main/contributing/).
