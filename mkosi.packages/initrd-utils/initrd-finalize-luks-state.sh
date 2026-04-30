#!/bin/sh

# Check if we were able to automatically unlock the root LUKS volume.
if systemctl status systemd-cryptsetup@root.service > /dev/null 2>&1; then
    # Clear any existing UEFI variable
    if [ -e /sys/firmware/efi/efivars/IncusOSTPMState-12f075e0-2d07-493d-811a-00920a72c04c ]; then
        chattr -i /sys/firmware/efi/efivars/IncusOSTPMState-12f075e0-2d07-493d-811a-00920a72c04c
        rm /sys/firmware/efi/efivars/IncusOSTPMState-12f075e0-2d07-493d-811a-00920a72c04c
    fi

    if journalctl -b -g "TPM2 operation failed, falling back to traditional unlocking" -u systemd-cryptsetup@root.service > /dev/null 2>&1; then
        # A recovery password was used to unlock the volume
        printf "\07\00\00\00\00\00\00\01" > /sys/firmware/efi/efivars/IncusOSTPMState-12f075e0-2d07-493d-811a-00920a72c04c
    else
        # The TPM was able to unlock the volume
        printf "\07\00\00\00\00\00\00\00" > /sys/firmware/efi/efivars/IncusOSTPMState-12f075e0-2d07-493d-811a-00920a72c04c
    fi
fi

# If we are using swtpm, extend PCR15's value before we exit the initrd.
if [ -d /boot/swtpm/ ]; then
    /usr/bin/initrd-utils seal-pcr15

    /usr/bin/swtpm_ioctl --unix /run/swtpm.sock -v
fi
