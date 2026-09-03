#!/bin/sh
set -eu

# Move systemd unit enablement from /etc to /usr so the system services don't
# depend on /etc/systemd, which is wiped on boot by the initrd root cleanup.
cp -a "${BUILDROOT}/etc/systemd/system/." "${BUILDROOT}/usr/lib/systemd/system/"
rm -rf "${BUILDROOT}/etc/systemd/system"
mkdir "${BUILDROOT}/etc/systemd/system"
