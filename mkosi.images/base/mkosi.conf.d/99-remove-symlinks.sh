#!/bin/sh -eux
mkdir -p "${DESTDIR}/usr/bin" "${DESTDIR}/usr/sbin"
rm -f \
    "${DESTDIR}/usr/bin/awk" \
    "${DESTDIR}/usr/bin/which" \
    "${DESTDIR}/usr/sbin/ovs-vswitchd"

ln -s "/usr/bin/mawk" "${DESTDIR}/usr/bin/awk"
ln -s "/usr/bin/which.debianutils" "${DESTDIR}/usr/bin/which"
ln -s "/usr/lib/openvswitch-switch/ovs-vswitchd" "${DESTDIR}/usr/sbin/ovs-vswitchd"

exit 0
