#!/bin/sh

set -eu

[ "$1" = "final" ] || exit 0

# Install the packages.
apt-get update

# FIXME: Pinning to older version to avoid breakage on arm64.
version="20.2.2-pve1"
apt-get install --yes "ceph-common=${version}" \
    "libcephfs2=${version}" \
    "librados2=${version}" \
    "libradosstriper1=${version}" \
    "librbd1=${version}" \
    "librgw2=${version}" \
    "python3-ceph-argparse=${version}" \
    "python3-ceph-common=${version}" \
    "python3-cephfs=${version}" \
    "python3-rados=${version}" \
    "python3-rbd=${version}" \
    "python3-rgw=${version}"

exit 0
