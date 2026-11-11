#!/bin/sh -eux

KERNEL="$(ls /buildroot/usr/lib/modules/)"

# Install the drbd sources
mkdir -p /run/lock
apt-get install --no-install-recommends --yes drbd-dkms || true

cp "/buildroot/boot/config-${KERNEL}" "/buildroot/usr/src/linux-headers-${KERNEL}/.config"
apt-get install --no-install-recommends --yes drbd-dkms

# Sign the module
mkdir -p "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/"

for mod in drbd drbd_transport_tcp drbd_transport_lb-tcp drbd_transport_rdma; do
    [ -e "/buildroot/usr/lib/modules/${KERNEL}/updates/dkms/${mod}.ko" ] || continue

    "/buildroot/usr/src/linux-headers-${KERNEL}/scripts/sign-file" \
        sha256 /work/src/mkosi.key /work/src/mkosi.crt \
        "/buildroot/usr/lib/modules/${KERNEL}/updates/dkms/${mod}.ko" \
        "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/${mod}.ko"
done

# Keep the copyright file so the module is covered by the license information.
mkdir -p "${DESTDIR}/usr/share/doc/drbd-dkms/"
cp /buildroot/usr/share/doc/drbd-dkms/copyright "${DESTDIR}/usr/share/doc/drbd-dkms/"
