% Include content from [../CONTRIBUTING.md](../CONTRIBUTING.md)
```{include} ../CONTRIBUTING.md
```

## Building locally
You can build IncusOS locally. Only users specifically interested in the
development and testing of new IncusOS features should need to do this.

We currently only support building IncusOS from a Debian 13 system
though other Debian releases and Debian derivatives like Ubuntu may work
as well.

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

You can also easily side-load a custom `incus-osd` binary into the virtual machine.
Execution from the root disk isn't allowed, so a memory file system is required:

    incus exec test-incus-os -- mkdir -p /root/dev/
    incus exec test-incus-os -- mount -t tmpfs tmpfs /root/dev/

Once that's in place, you can build the new binary:

    cd ./incus-osd/
    go build ./cmd/incus-osd/

And finally put it in place and have the system run it:

    incus exec test-incus-os -- umount -l /usr/local/bin/incus-osd || true
    incus exec test-incus-os -- rm -f /root/dev/incus-osd
    incus file push ./incus-osd test-incus-os/root/dev/
    incus exec test-incus-os -- mount -o bind /root/dev/incus-osd /usr/local/bin/incus-osd
    incus exec test-incus-os -- pkill -9 incus-osd

Those last two steps can be repeated every time you want to build and run a new binary.
The first step must be run every time the system is restarted.

When debugging, it's a good idea to install the `debug` application which contains a variety of useful tools, including a basic text editor (`nano`).

    incus exec test-incus-os bash
    curl --unix-socket /run/incus-os/unix.socket socket/1.0/applications -X POST -d '{"name": "debug"}'
