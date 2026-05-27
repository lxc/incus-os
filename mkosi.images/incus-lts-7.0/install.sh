#!/bin/sh

set -eu

[ "$1" = "final" ] || exit 0

# This is a terrible hack. mkosi doesn't support subimage-specific sandboxes,
# which would be the most correct solution as we could cleanly define an apt
# pinning configuration for this specific subimage. Alternatively, if apt
# supported getting pinning configuration from the command line, we could
# add that in a slightly-ugly apt-get command. However, we're stuck with creating
# the pinning configuration under /run/, since the actual file system that
# we have access to at this stage is read-only. It works, but I don't like it.

mkdir -p /run/apt/preferences.d/
cat <<EOF > /run/apt/preferences.d/zabbly
Package: *
Pin: release l=incus-lts-7.0
Pin-Priority: 900
EOF

# Install the incus packages.
apt-get -o Dir::Etc::PreferencesParts="/run/apt/preferences.d/" install incus incus-ui-canonical --yes

exit 0
