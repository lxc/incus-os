#!/bin/sh
set -eu

# The module index was regenerated without the GPU drivers shipped in gpu-support, restore the one from the base image.
KERNEL="$(ls "${BUILDROOT}/usr/lib/modules/")"
cp -p "${SRCDIR}/mkosi.output/base/usr/lib/modules/${KERNEL}"/modules.* "${BUILDROOT}/usr/lib/modules/${KERNEL}/"
