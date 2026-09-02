#!/bin/sh

# Re-enable the OSD services
for d in /var/lib/ceph/osd/ceph-*; do
    systemctl enable --now "ceph-osd@${d##*ceph-}"
done
