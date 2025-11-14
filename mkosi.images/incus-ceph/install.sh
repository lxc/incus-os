#!/bin/sh
[ "$1" = "final" ] || exit 0

# Install the packages.
apt-get install ceph-common --yes

exit 0
