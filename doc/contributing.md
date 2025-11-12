% Include content from [../CONTRIBUTING.md](../CONTRIBUTING.md)
```{include} ../CONTRIBUTING.md
```

## Building locally
You can build IncusOS locally. Only users specifically interested in the
development and testing of new IncusOS features should need to do this.
Building your own images requires a current version of `mkosi`, and should work
on most Linux distributions, with Debian/Ubuntu being the most well-tested.

After cloning the repository from GitHub, simply run:

    make

By default the build will produce a raw image in the `mkosi.output/` directory,
suitable for writing to a USB stick. It is also possible to build an ISO
image if you need to boot from a (virtual) CD-ROM device:

    make build-iso

## Testing
To test a locally built raw image in an Incus virtual machine, run:

    make test

After IncusOS has completed its installation and is running in the virtual machine, to load
applications run:

    make test-applications

To test the update process, build a new image and update to it with:

    make
    make test-update

## Debugging

When IncusOS is run in an Incus virtual machine, it is possible to `exec` into the running
system to facilitate debugging of the system:

    incus exec test-incus-os bash

You can also easily side-load a custom `incus-osd` binary into the virtual machine:

    cd ./incus-osd/
    go build ./cmd/incus-osd/
    incus file push ./incus-osd test-incus-os/root/

Then `exec` into the virtual machine, stop the main `incus-osd` service and run the local copy:

    incus exec test-incus-os bash
    systemctl stop incus-osd
    mount -o bind /root/incus-osd /usr/local/bin/incus-osd
    systemctl start incus-osd

There's no text editor in the base image, but it's easy to fetch the [micro](https://github.com/zyedidia/micro) text editor:

    mkdir /tmp/micro/
    curl -sLo /tmp/micro/micro.tar.gz https://github.com/zyedidia/micro/releases/download/v2.0.14/micro-2.0.14-linux64-static.tar.gz
    tar -xzf /tmp/micro/micro.tar.gz -C /tmp/micro --strip-components=1
    mv /tmp/micro/micro /root/micro
    rm -rf /tmp/micro
    export EDITOR=/root/micro
