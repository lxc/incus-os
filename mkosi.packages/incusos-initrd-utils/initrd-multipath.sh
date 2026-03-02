#!/bin/sh

# If the fnic kernel driver is loaded (used by Cisco FCoE HBA), sleep a few seconds to fully allow drives to come online.
if lsmod | grep fnic > /dev/null 2>&1; then
    sleep 5
fi

# Only attempt to activate root device multipath if there are duplicate WWNs present.
if [ ! -e "/dev/disk/by-partlabel/esp" ]; then
    exit
fi

ROOT_DEVICE=$(lsblk -o pkname -dn /dev/disk/by-partlabel/esp)

# Don't proceed if no WWN exists for the ESP partition, for example when booting from a CDROM.
if [ "$ROOT_DEVICE" = "" ]; then
    exit
fi

ROOT_DEVICE="/dev/$ROOT_DEVICE"
ROOT_WWN=$(lsblk -o WWN -dn "$ROOT_DEVICE")
TOTAL_ROOT_WWNS=$(lsblk -o WWN -dn | grep -v "^$" | grep -c "^$ROOT_WWN$")

if [ "$TOTAL_ROOT_WWNS" -gt 1 ]; then
    multipath -a "$ROOT_DEVICE"
    multipath -r

    # Need to sleep a few seconds to allow the multipath device to become available.
    sleep 5
fi
