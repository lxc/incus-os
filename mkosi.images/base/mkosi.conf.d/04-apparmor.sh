#!/bin/sh -eux

# Copy apparmor configuration to /usr/share/.
mkdir -p "${DESTDIR}/usr/share/"
cp -r /buildroot/etc/apparmor/ "${DESTDIR}/usr/share/"
cp -r /buildroot/etc/apparmor.d/ "${DESTDIR}/usr/share/"

exit 0
