#!/bin/sh
KERNEL="$(ls /buildroot/usr/lib/modules/)"

mkdir -p "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/"

"/buildroot/usr/src/linux-headers-${KERNEL}/scripts/sign-file" \
    sha256 /work/src/mkosi.key /work/src/mkosi.crt \
    "/buildroot/usr/lib/modules/${KERNEL}/updates/dkms/spl.ko" \
    "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/spl.ko"

"/buildroot/usr/src/linux-headers-${KERNEL}/scripts/sign-file" \
    sha256 /work/src/mkosi.key /work/src/mkosi.crt \
    "/buildroot/usr/lib/modules/${KERNEL}/updates/dkms/zfs.ko" \
    "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/zfs.ko"
