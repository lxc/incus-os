#!/bin/sh
# shellcheck disable=SC1083

# Wipe the disk
wipefs -af /dev/ceph-1

# Get a new OSD ID
ID=$(ceph osd create)

# Create filesystem structure
mkdir -p "/var/lib/ceph/osd/ceph-${ID}"
ln -s /dev/ceph-1 "/var/lib/ceph/osd/ceph-${ID}/block"
uuidgen > "/var/lib/ceph/osd/ceph-${ID}/fsid"
echo bluestore > "/var/lib/ceph/osd/ceph-${ID}/type"

# Keyring generation
ceph auth add "osd.${ID}" mgr 'allow profile osd' mon 'allow profile osd' osd 'allow *'
ceph auth export "osd.${ID}" > "/var/lib/ceph/osd/ceph-${ID}/keyring"

# Initialize the OSD
ceph-osd --mkfs --no-mon-config -i "${ID}"

# Fix permissions
chown -R ceph:ceph /var/lib/ceph/osd/

# Start the OSD
systemctl enable --now "ceph-osd@${ID}"

# Set device type
sleep 5s
ceph osd crush rm-device-class "${ID}"
ceph osd crush set-device-class {{.DEVICE_CLASS}} "${ID}"
