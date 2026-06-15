#!/bin/sh
# shellcheck disable=SC1083

# Setup the keyrings
ceph mon getmap > /tmp/monmap

# Setup the monitor
mkdir -p /var/lib/ceph/mon/ceph-{{.INST_NAME}}
ceph-mon --mkfs -i {{.INST_NAME}} --monmap /tmp/monmap --keyring /tmp/ceph.mon.keyring
chown -R ceph:ceph /var/lib/ceph/mon/ceph-{{.INST_NAME}}
systemctl enable --now ceph-mon@{{.INST_NAME}}.service

# Setup the manager
mkdir -p /var/lib/ceph/mgr/ceph-{{.INST_NAME}}
ceph auth get-or-create mgr.{{.INST_NAME}} mon 'allow profile mgr' osd 'allow *' mds 'allow *' > /var/lib/ceph/mgr/ceph-{{.INST_NAME}}/keyring
chown -R ceph:ceph /var/lib/ceph/mgr/ceph-{{.INST_NAME}}
systemctl enable --now ceph-mgr@{{.INST_NAME}}.service

# Setup the metadata service
mkdir -p /var/lib/ceph/mds/ceph-{{.INST_NAME}}
ceph auth get-or-create mds.{{.INST_NAME}} mon 'profile mds' mgr 'profile mds' mds 'allow *' osd 'allow *' > /var/lib/ceph/mds/ceph-{{.INST_NAME}}/keyring
chown -R ceph:ceph /var/lib/ceph/mds/ceph-{{.INST_NAME}}
systemctl enable --now ceph-mds@{{.INST_NAME}}.service

# Setup the block device mirroring service
ceph auth get-or-create client.rbd-mirror.{{.INST_NAME}} mon 'profile rbd-mirror' osd 'profile rbd' > /etc/ceph/ceph.client.rbd-mirror.{{.INST_NAME}}.keyring
chown ceph:ceph /etc/ceph/ceph.client.rbd-mirror.{{.INST_NAME}}.keyring
systemctl enable --now ceph-rbd-mirror@rbd-mirror.{{.INST_NAME}}.service
