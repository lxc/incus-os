#!/bin/sh
# shellcheck disable=SC1083

# Find the OSD backed by the device, stop it and remove its local state.
for d in /var/lib/ceph/osd/ceph-*; do
    [ "$(readlink "${d}/block")" = "{{.DEVICE_PATH}}" ] || continue

    ID="${d##*ceph-}"

    systemctl disable --now "ceph-osd@${ID}"
    rm -rf "${d}"

    exit 0
done

echo "No OSD found using {{.DEVICE_PATH}}" >&2
exit 1
