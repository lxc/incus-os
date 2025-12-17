#!/bin/sh -eux

# Copy protocols and services configuration to /usr/share/.
mkdir -p "${DESTDIR}/usr/share/"
cp /buildroot/etc/protocols "${DESTDIR}/usr/share/"
cp /buildroot/etc/services "${DESTDIR}/usr/share/"

exit 0
