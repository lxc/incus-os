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

# Originally we had been loop-mounting the raw image and just copying in the
# needed secure boot certificates. However, occasionally the mount failed in
# CI runs. So, switch to using mtools to directly manipulate the ESP vfat
# partition in the image so we don't need to mount anything.

# This is the offset to the beginning of the ESP partition.
OFFSET=1048576

# Remove any existing certificates we will be overwriting.
mdeltree -i "$1"@@$OFFSET ::loader/keys/auto/ || true
mdeltree -i "$1"@@$OFFSET ::keys/ || true
mdel -i "$1"@@$OFFSET ::mkosi.der || true

# Push the new enrollment keys.
mmd -i "$1"@@$OFFSET ::loader/keys/auto
mcopy -i "$1"@@$OFFSET certs/efi/*.auth ::loader/keys/auto/

# Push the keys as DER.
mmd -i "$1"@@$OFFSET ::keys
mcopy -i "$1"@@$OFFSET certs/efi/*.der ::keys/ || true
