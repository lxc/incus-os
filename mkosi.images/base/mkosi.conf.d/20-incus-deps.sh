#!/bin/sh -eux
[ "$1" = "final" ] || exit 0

# Install the incus packages.
apt-get update
apt-get install incus incus-ui-canonical --yes

# Mark everything left as manually installed.
dpkg -l | grep ^ii | awk '{print $2}' | xargs apt-mark manual

# Then remove incus itself.
apt-get remove --purge incus incus-base incus-client incus-ui-canonical --yes

exit 0
