#!/bin/sh

# Manually setup the root multipath device, if detected.
for MP in $(dmsetup ls --target multipath | cut -f1); do
    # Skip any multipath devices that don't have at least two partitions.
    if [ ! -e "/dev/mapper/${MP}-part1" ] || [ ! -e "/dev/mapper/${MP}-part2" ]; then
        continue
    fi

    FIRST_PART_LABEL=$(lsblk -o partlabel -dn "/dev/mapper/${MP}-part1")
    SECOND_PART_LABEL=$(lsblk -o partlabel -dn "/dev/mapper/${MP}-part2")

    # Skip any multipath device that doesn't have expected IncusOS partition labels.
    if [ "$FIRST_PART_LABEL" != "esp" ] || [ "$SECOND_PART_LABEL" != "seed-data" ]; then
        continue
    fi

    # If partition 8 exists, but partition 9 doesn't, this is the first boot, and we need
    # to manually run systemd-repart to create the final three partitions. systemd doesn't
    # support disks with multiple backing devices (like mulitpath), so our normal reliance
    # on the systemd-repart service won't work.
    if [ ! -e "/dev/mapper/${MP}-part9" ] && [ -e "/dev/mapper/${MP}-part8" ]; then
        systemd-repart --dry-run=no --definitions=/usr/lib/incus-os/repart.d/ "/dev/mapper/${MP}"

        kpartx -p "-part" -a "/dev/mapper/${MP}"
    fi

    # Attempt to unlock swap and root partitions.
    if [ -e "/dev/mapper/${MP}-part9" ]; then
        # Unlock root partition, which will be automatically detected and then mounted.
        systemd-cryptsetup attach "root" "/dev/mapper/${MP}-part10" "" "tpm2-device=auto,tpm2-measure-pcr=yes,tries=0"

        # Unlock swap and manually activate since it's not automatically picked up by systemd.
        systemd-cryptsetup attach "swap" "/dev/mapper/${MP}-part9" "" "tpm2-device=auto,tpm2-measure-pcr=yes,tries=0"
        swapon /dev/mapper/swap
    fi

    break
done
