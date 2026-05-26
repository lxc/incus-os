#!/bin/sh

set -eu

[ "$1" = "final" ] || exit 0

# Install the incus packages.
apt-get update
apt-get install incus incus-ui-canonical --yes

# Install additional dependencies/recommends.
apt-get install btrfs-progs xfsprogs --yes

exit 0
