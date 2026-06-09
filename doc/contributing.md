% Include content from [../CONTRIBUTING.md](../CONTRIBUTING.md)
```{include} ../CONTRIBUTING.md
```

## Building locally
You can build IncusOS locally. Only users specifically interested in the
development and testing of new IncusOS features should need to do this.

We currently only support building IncusOS in a Debian 13 environment,
though other Debian releases and Debian derivatives like Ubuntu may work
as well.

Begin by ensuring that the required build dependencies are installed:

    sudo apt install devscripts golang-any libnbd-dev mkosi

When building locally, during the initial build various development Secure
Boot certificates will be generated so the resulting build artifacts can be
properly signed. This should be a transparent process, and only needs to be
done once. If for some reason your development Secure Boot certificates get
messed up, you can regenerate them by running:

    rm -rf ./certs/ ./incus-osd/certs/files/
    make generate-test-certs

From the root of the IncusOS repository, a new build can be created by running:

    make

By default, the build will produce a raw image in the `mkosi.output/` directory,
suitable for writing to a USB stick. It is also possible to build an ISO
image if you need to boot from a (virtual) CD-ROM device:

    make build-iso

## Testing

The recommended way to test IncusOS is running it within an Incus virtual machine.
It is also possible to test IncusOS on a physical machine, but debugging and
introspecting into the system state will be greatly restricted.

```{note}
When testing IncusOS in an Incus virtual machine, the following assumptions are made:

* Incus is configured to run on the local machine; remote Incus servers aren't supported

* The Incus client has full administrative access (most commonly, your user is part of
the `incus-admin` group)

* The default `incusbr0` network bridge exists, and will be used by the virtual machines
for their network connectivity.
```

To support local development builds of IncusOS, a Python-based HTTP server will be
started as needed to provide a very simple private images server. This local images
server operates in the same way as the public Linux Containers images server; the virtual
machine's `images` provider will be automatically configured to check locally for any
available updates.

To test a locally built raw image in an Incus virtual machine, run:

    make test

To test the update process, build a new image and update to it with:

    make
    make test-update

A new build can also be published locally by running:

    make publish-local-update

## Extending the `cli` package

IncusOS provides a `cli` package which is imported by Incus, Migration Manager and Operations Center to provide an end-user command line interface to IncusOS.

When adding a new command or making some other change, it's easy to test the changes by building a local client binary. Assuming you already have
the IncusOS repository cloned in your home directory, clone the Incus repository and create a symlink to the IncusOS `cli` package at the root:

    git clone https://github.com/lxc/incus
    cd incus/
    ln -s ~/incus-os/incus-osd/cli/ .

Then, update the import path of `cmd/incus/admin_os.go` from `github.com/lxc/incus-os/incus-osd/cli` to `github.com/lxc/incus/v7/cli`.

Finally, build the local Incus client:

    go build ./cmd/incus/

You can now test your new command or other changes: `./incus admin os ...`

## Debugging

```{note}
Running commands or opening a shell on an IncusOS server is only possible when run as an Incus virtual machine and requires fully enabling `incus-agent`
support. Be aware that this will cause the system to report a degraded security state via the API and an on-screen message. This is because running
arbitrary commands within the virtual machine can make changes outside of the control of IncusOS.

To fully enable `incus-agent` support in the virtual machine, run the following command and then restart the virtual machine.

    incus config set <vm> systemd.credential.fully-enable-incus-agent=true
```

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
