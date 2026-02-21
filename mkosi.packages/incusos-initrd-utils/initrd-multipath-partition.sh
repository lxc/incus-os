#!/bin/sh

# Manually setup any detected multipath devices.
for MP in $(dmsetup ls --target multipath | cut -f1); do
    # If partition 8 exists, but partition 9 doesn't, this is likely first boot, and we need
    # to manually run systemd-repart to create the final three partitions. systemd doesn't
    # support disks with multiple backing devices (like mulitpath), so our normal reliance
    # on the systemd-repart service won't work.
    if [ ! -e "/dev/mapper/${MP}-part9" ] && [ -e "/dev/mapper/${MP}-part8" ]; then
        systemd-repart --dry-run=no --definitions=/usr/lib/incus-os/repart.d/ "/dev/mapper/${MP}"

        kpartx -p "-part" -a "/dev/mapper/${MP}"
    fi

    # Attempt to unlock swap and root partitions.
    if [ -e "/dev/mapper/${MP}-part9" ]; then
        # Unlock swap and manually activate since it's not automatically picked up by systemd.
        systemd-cryptsetup attach swap "/dev/mapper/${MP}-part9"
        swapon /dev/mapper/swap

        # Unlock root partition, which will be automatically detected and then mounted.
        systemd-cryptsetup attach root "/dev/mapper/${MP}-part10"
    fi
done
