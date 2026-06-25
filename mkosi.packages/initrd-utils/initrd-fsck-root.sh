#!/bin/sh

# Ideally systemd-fsck-root.service would check the root partition automatically, but because of
# how we are relying on swtpm storing state in /boot/, the unencrypted root device won't be available.
# Attempting to add additional service ordering dependencies results in a dependency cycle, so we mimic
# what that service does immediately following the successful unlocking of the root device.

/usr/sbin/fsck.ext4 -p /dev/mapper/root
ret=$?

# Perform a couple actions based on the exit code of fsck. If it failed to repair the file system, don't
# cause this script to exit with a failure. Hopefully the file system can still be mounted, otherwise
# sysroot.mount will likely fail.
if [ $ret = 0 ] || [ $ret = 1 ]; then
    # No errors or errors were corrected
    mkdir -p /run/initramfs/
    touch /run/initramfs/fsck-root
elif [ $ret = 2 ]; then
    # A system reboot is required
    systemctl reboot
fi
