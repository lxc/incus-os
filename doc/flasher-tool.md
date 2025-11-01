# Flasher tool
The [flasher tool](https://github.com/lxc/incus-os/tree/main/incus-osd/cmd/flasher-tool)
is a user-friendly way to provide install configuration for Incus OS.

## Usage

    go build ./cmd/flasher-tool/
    ./flasher-tool

You will first be prompted for the image format you want to use, either iso
(default) or raw image (img). Note that the iso isn't a hybrid image; if you
want to boot from a USB stick you should choose the img format.

The flasher tool will then connect to the Linux Containers CDN and download the
latest release.

Once downloaded, you will be presented with an interactive menu you can use to
customize the install options.

After writing the image and exiting, you can then install Incus OS from the
resulting image.

## Environment variables

Three special environment variables are recognized by the flasher tool, which can be
used to provide defaults:

- `INCUSOS_IMAGE`: Specifies a local Incus OS install image to work with, and will
  disable checking the Linux Containers CDN for a newer version.

- `INCUSOS_IMAGE_FORMAT`: When downloading from the Linux Containers CDN, specifies
  the Incus OS install image format (`iso` or `img`) to fetch, and will disable
  prompting the user for this information.

- `INCUSOS_SEED_TAR`: Specifies a user-created [install seed](install-seed.md)
  archive to write to the install image. Disables all prompting of the user, and is
  considered an advanced option.
