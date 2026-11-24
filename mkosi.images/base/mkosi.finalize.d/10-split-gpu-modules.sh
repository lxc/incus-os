#!/bin/sh
set -eu

KERNEL="$(ls "${BUILDROOT}/usr/lib/modules/")"
SOURCE="${BUILDROOT}/usr/lib/modules/${KERNEL}"
TARGET="${OUTPUTDIR}/base-gpu-modules/usr/lib/modules/${KERNEL}"

# Applications can't replace the module index, so generate it while the GPU drivers are still present.
depmod -a -b "${BUILDROOT}/usr" "${KERNEL}"

# The GPU drivers themselves are shipped in the gpu-support application.
for dir in kernel/drivers/accel/ivpu kernel/drivers/gpu/drm/amd kernel/drivers/gpu/drm/i915 kernel/drivers/gpu/drm/nouveau kernel/drivers/gpu/drm/xe updates/nvidia; do
    # Not all drivers exist on all architectures.
    [ -d "${SOURCE}/${dir}" ] || continue

    mkdir -p "${TARGET}/$(dirname "${dir}")"
    mv "${SOURCE}/${dir}" "${TARGET}/${dir}"
done
