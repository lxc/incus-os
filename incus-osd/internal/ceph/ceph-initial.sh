#!/bin/sh
# shellcheck disable=SC1083

# Create ceph.conf
(
cat << EOG
[global]
fsid = {{.FSID}}
mon_initial_members = {{.INST_NAME}}
mon_host = [{{.INST_IPV6}}]
ms_bind_ipv4 = false
ms_bind_ipv6 = true
public_network = {{.NET_IPV6}}
EOG
) > /etc/ceph/ceph.conf

# Create the initial keyrings and map
ceph-authtool --create-keyring /tmp/ceph.mon.keyring --gen-key -n mon. --cap mon 'allow *'
ceph-authtool --create-keyring /etc/ceph/ceph.client.admin.keyring --gen-key -n client.admin --cap mon 'allow *' --cap osd 'allow *'  --cap mds 'allow *' --cap mgr 'allow *'
ceph-authtool /tmp/ceph.mon.keyring --import-keyring /etc/ceph/ceph.client.admin.keyring
chown ceph:ceph /tmp/ceph.mon.keyring
monmaptool --create --add {{.INST_NAME}} [{{.INST_IPV6}}] --fsid {{.FSID}} /tmp/monmap

# Setup the initial monitor
mkdir -p /var/lib/ceph/mon/ceph-{{.INST_NAME}}
ceph-mon --mkfs -i {{.INST_NAME}} --monmap /tmp/monmap --keyring /tmp/ceph.mon.keyring
chown -R ceph:ceph /var/lib/ceph/mon/ceph-{{.INST_NAME}}
systemctl enable --now ceph-mon@{{.INST_NAME}}.service

# Apply some initial configuration
ceph mon enable-msgr2
ceph config set global auth_allow_insecure_global_id_reclaim false
ceph config set global mon_allow_pool_delete true

# Setup the initial manager
mkdir -p /var/lib/ceph/mgr/ceph-{{.INST_NAME}}
ceph auth get-or-create mgr.{{.INST_NAME}} mon 'allow profile mgr' osd 'allow *' mds 'allow *' > /var/lib/ceph/mgr/ceph-{{.INST_NAME}}/keyring
chown -R ceph:ceph /var/lib/ceph/mgr/ceph-{{.INST_NAME}}
systemctl enable --now ceph-mgr@{{.INST_NAME}}.service

# Setup the initial metadata service
mkdir -p /var/lib/ceph/mds/ceph-{{.INST_NAME}}
ceph auth get-or-create mds.{{.INST_NAME}} mon 'profile mds' mgr 'profile mds' mds 'allow *' osd 'allow *' > /var/lib/ceph/mds/ceph-{{.INST_NAME}}/keyring
chown -R ceph:ceph /var/lib/ceph/mds/ceph-{{.INST_NAME}}
systemctl enable --now ceph-mds@{{.INST_NAME}}.service

# Setup the initial block device mirroring service
ceph auth get-or-create client.rbd-mirror.{{.INST_NAME}} mon 'profile rbd-mirror' osd 'profile rbd' > /etc/ceph/ceph.client.rbd-mirror.{{.INST_NAME}}.keyring
chown ceph:ceph /etc/ceph/ceph.client.rbd-mirror.{{.INST_NAME}}.keyring
systemctl enable --now ceph-rbd-mirror@rbd-mirror.{{.INST_NAME}}.service
