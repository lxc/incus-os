#!/bin/sh

# Ceph can only identify an OSD device through its kernel name, so have each OSD link it on startup.
mkdir -p /usr/local/sbin /etc/systemd/system/ceph-osd@.service.d

cat > /usr/local/sbin/ceph-osd-device-name << 'EOF'
#!/bin/sh
set -eu

DEV="/var/lib/ceph/osd/ceph-${1}/block"
NAME="$(sed -n "s/^DEVNAME=//p" "/sys/dev/block/$((0x$(stat -L -c %t "${DEV}"))):$((0x$(stat -L -c %T "${DEV}")))/uevent")"

[ -e "/dev/${NAME}" ] || ln -s "$(readlink "${DEV}")" "/dev/${NAME}"
EOF

chmod +x /usr/local/sbin/ceph-osd-device-name

cat > /etc/systemd/system/ceph-osd@.service.d/device-name.conf << 'EOF'
[Service]
ExecStartPre=-/usr/local/sbin/ceph-osd-device-name %i
EOF

systemctl daemon-reload
