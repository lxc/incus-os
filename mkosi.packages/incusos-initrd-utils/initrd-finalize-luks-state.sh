#!/bin/sh

# If we are using swtpm, extend PCR15's value before we exit the initrd.
if [ -d /boot/swtpm/ ]; then
    /usr/bin/incusos-initrd-utils seal-pcr15

    /usr/bin/swtpm_ioctl --unix /run/swtpm.sock -v
fi
