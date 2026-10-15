#!/bin/sh

# Inject a read-only random seed file into the ESP of the final image.
# systemd-boot skips random seed handling when the file has the read-only
# attribute, which avoids writing to the install media. Some firmwares
# mishandle failed writes to read-only media and then refuse to boot.

set -e

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <input img>"
    exit 1
fi

# This is the offset to the beginning of the ESP partition.
OFFSET=1048576

SEED=$(mktemp)
head -c 32 /dev/urandom > "${SEED}"

mdel -i "$1"@@$OFFSET ::loader/random-seed 2>/dev/null || true
mcopy -i "$1"@@$OFFSET "${SEED}" ::loader/random-seed
mattrib -i "$1"@@$OFFSET +r ::loader/random-seed

rm -f "${SEED}"
