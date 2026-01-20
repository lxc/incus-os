#!/bin/sh
if [ "${container:-}" != "mkosi" ]; then
    exec mkosi-chroot "${CHROOT_SCRIPT}" "${@}"
fi

# Check if any action needed.
[ ! -e "/work/src/patches/edk2-logo.bmp" ] && exit 0

# Swap the EDK2 logo
set -x
cd /work/src/app-build/edk2

ARCH="X64"
PKG="OvmfPkg/OvmfPkgX64.dsc"
if [ "$(uname -m)" = "aarch64" ]; then
    ARCH="AARCH64"
    PKG="ArmVirtPkg/ArmVirtQemu.dsc"
fi

# shellcheck disable=SC1091
. ./edksetup.sh
set -eu

LOGO_DXE_GUID="F74D20EE-37E7-48FC-97F7-9B1047749C69"
cp /work/src/patches/edk2-logo.bmp MdeModulePkg/Logo/Logo.bmp

make -C "BaseTools" "ARCH=${ARCH}"
build -m "MdeModulePkg/Logo/LogoDxe.inf" \
      -a "${ARCH}" -t "GCC5" -b "RELEASE" -p "${PKG}"

# shellcheck disable=SC2046
LOGO_DXE_FFS=$(ls -1 Build/*/*/FV/Ffs/${LOGO_DXE_GUID}LogoDxe/${LOGO_DXE_GUID}.ffs)

PYTHONPATH=$(pwd)/BaseTools/Source/Python
export PYTHONPATH

mkdir -p "${DESTDIR}/opt/incus/share/qemu/"
python3 BaseTools/Source/Python/FMMT/FMMT.py \
    -r "/opt/incus/share/qemu/OVMF_CODE.4MB.fd" \
    "${LOGO_DXE_GUID=}" \
    "${LOGO_DXE_FFS}" \
    "${DESTDIR}/opt/incus/share/qemu/OVMF_CODE.4MB.fd"

exit 0
