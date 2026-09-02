#!/bin/sh
# shellcheck disable=SC1083

# Setup the object gateway on containers predating its introduction
# This can be removed after 2026-12-01
if [ ! -e /var/lib/ceph/radosgw/ceph-rgw.{{.INST_NAME}}/keyring ]; then
    mkdir -p /var/lib/ceph/radosgw/ceph-rgw.{{.INST_NAME}}
    ceph auth get-or-create client.rgw.{{.INST_NAME}} mon 'allow rw' osd 'allow rwx' > /var/lib/ceph/radosgw/ceph-rgw.{{.INST_NAME}}/keyring
    chown -R ceph:ceph /var/lib/ceph/radosgw/ceph-rgw.{{.INST_NAME}}
fi

# Re-enable the systemd services
systemctl enable --now ceph-mon@{{.INST_NAME}}.service
systemctl enable --now ceph-mgr@{{.INST_NAME}}.service
systemctl enable --now ceph-mds@{{.INST_NAME}}.service
systemctl enable --now ceph-rbd-mirror@rbd-mirror.{{.INST_NAME}}.service
systemctl enable --now ceph-radosgw@rgw.{{.INST_NAME}}.service
