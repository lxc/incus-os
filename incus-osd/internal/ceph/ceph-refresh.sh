#!/bin/sh
# shellcheck disable=SC1083

# Re-enable the systemd services
systemctl enable --now ceph-mon@{{.INST_NAME}}.service
systemctl enable --now ceph-mgr@{{.INST_NAME}}.service
systemctl enable --now ceph-mds@{{.INST_NAME}}.service
systemctl enable --now ceph-rbd-mirror@rbd-mirror.{{.INST_NAME}}.service
