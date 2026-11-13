#!/bin/sh
if [ "${container:-}" != "mkosi" ]; then
    exec mkosi-chroot "${CHROOT_SCRIPT}" "${@}"
fi

set -eux

KERNEL="$(ls /usr/lib/modules/)"
HEADERS="/usr/src/linux-headers-${KERNEL}"

# Build the NVIDIA open kernel modules from a copy of the sources.
rm -rf /tmp/nvidia-modules
cp -a /work/src/app-build/nvidia-open-gpu-kernel-modules /tmp/nvidia-modules
make -C /tmp/nvidia-modules modules -j"$(nproc)" KERNEL_UNAME="${KERNEL}" SYSSRC="${HEADERS}"

# Sign the modules.
mkdir -p "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/nvidia/"

for mod in /tmp/nvidia-modules/kernel-open/*.ko; do
    strip --strip-debug "${mod}"

    "${HEADERS}/scripts/sign-file" \
        sha256 /work/src/mkosi.key /work/src/mkosi.crt \
        "${mod}" \
        "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/nvidia/$(basename "${mod}")"
done

rm -rf /tmp/nvidia-modules
