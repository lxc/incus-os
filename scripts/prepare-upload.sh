#!/bin/sh

set -eu

if [ -z "${1:-}" ]; then
    echo "Usage: ${0} VERSION"
    exit 1
fi

VERSION=${1}

mkdir upload

./incus-osd/generate-manifests ./

cp incus-osd/flasher-tool upload/

cp mkosi.output/debug.raw upload/
cp mkosi.output/gpu-support.raw upload/
cp mkosi.output/incus.raw upload/
cp mkosi.output/incus-lts-7.0.raw upload/
cp mkosi.output/incus-ceph.raw upload/
cp mkosi.output/incus-linstor.raw upload/
cp mkosi.output/migration-manager.raw upload/
cp mkosi.output/operations-center.raw upload/

OSNAME=$(grep "ImageId=" mkosi.conf | cut -d '=' -f 2)

cp mkosi.output/"${OSNAME}"_"${VERSION}".raw upload/"${OSNAME}"_"${VERSION}".img
cp mkosi.output/"${OSNAME}"_"${VERSION}".iso upload/"${OSNAME}"_"${VERSION}".iso || true
cp mkosi.output/"${OSNAME}"_"${VERSION}".efi upload/
cp mkosi.output/"${OSNAME}"_"${VERSION}".usr-*.raw upload/
