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
