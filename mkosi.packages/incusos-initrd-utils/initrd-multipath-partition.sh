#!/bin/sh

# Manually setup any detected multipath devices.
for MP in $(dmsetup ls --target multipath | cut -f1); do
    if [ ! -e "/dev/mapper/${MP}-part9" ] && [ -e "/dev/mapper/${MP}-part8" ]; then
        systemd-repart --dry-run=no --definitions=/foobar/repart.d/ "/dev/mapper/${MP}"

        kpartx -p "-part" -a "/dev/mapper/${MP}"
    fi

    if [ -e "/dev/mapper/${MP}-part9" ]; then
        systemd-cryptsetup attach swap "/dev/mapper/${MP}-part9"
        swapon /dev/mapper/swap

        systemd-cryptsetup attach root "/dev/mapper/${MP}-part10"
    fi
done
