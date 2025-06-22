#!/bin/sh

# Inject signed default EFI secure boot variables into the final image.
# There doesn't seem to be a nice mkosi hook to automate this.

set -e

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <input img>"
    exit 1
fi

if [ ! -d certs/ ]; then
    echo "Directory './certs/' doesn't exist, exiting"
    exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
     echo "This script must be run as root"
     exit 1
fi

mkdir -p certs/mnt/
LOOP=$(losetup --show -f -P "$1")
mount "${LOOP}p1" certs/mnt/
rm certs/mnt/loader/keys/auto/*
cp certs/efi/*.auth certs/mnt/loader/keys/auto/
umount certs/mnt/
losetup -d "${LOOP}"
