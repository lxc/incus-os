# Flasher tool
The [flasher tool](https://github.com/lxc/incus-os/tree/main/incus-osd/cmd/flasher-tool)
is a user-friendly way to provide install configuration for Incus OS.

## Usage

Either build locally, or download from the [Incus OS Releases](https://github.com/lxc/incus-os/releases)
page.

    ./flasher-tool
    
You will first be prompted for the image format you want to use, either iso
(default) or raw image (img).

The flasher tool will then connect to GitHub and download the latest release.

After downloading, you will be presented with an interactive menu you can use to
customize the install options.

After writing the image and exiting, you can then install Incus OS from the
resulting image.

## Environment variables

Three special environment variables are recognized by the flasher tool, which can be
used to provide defaults:

  * `INCUSOS_IMAGE`: Specifies a local Incus OS install image to work with, and will
  disable checking GitHub for a newer version.
  
  * `INCUSOS_IMAGE_FORMAT`: When downloading from GitHub, specifies the Incus OS
  install image format (`iso` or `img`) to fetch, and will disable prompting the
  user for this information.
  
  * `INCUSOS_SEED_TAR`: Specifies a user-created [install seed](install-seed.md)
  archive to write to the install image. Disables all prompting of the user, and is
  considered an advanced option.
