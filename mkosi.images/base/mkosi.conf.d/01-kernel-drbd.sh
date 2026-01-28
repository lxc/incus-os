#!/bin/sh
KERNEL="$(ls /buildroot/usr/lib/modules/)"

# Install the drbd sources
mkdir -p /run/lock
apt-get install --no-install-recommends --yes drbd-dkms || true

cp "/buildroot/boot/config-${KERNEL}" "/buildroot/usr/src/linux-headers-${KERNEL}/.config"
apt-get install --no-install-recommends --yes drbd-dkms

# Sign the module
mkdir -p "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/"

for mod in drbd drbd_transport_tcp drbd_transport_lb-tcp drbd_transport_rdma; do
    "/buildroot/usr/src/linux-headers-${KERNEL}/scripts/sign-file" \
        sha256 /work/src/mkosi.key /work/src/mkosi.crt \
        "/buildroot/usr/lib/modules/${KERNEL}/updates/dkms/${mod}.ko" \
        "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/${mod}.ko"
done
