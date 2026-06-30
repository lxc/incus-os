#!/bin/sh

set -eu

[ "$1" = "final" ] || exit 0

# Install the packages.
mkdir -p /run/lock
apt-get update
apt-get install bcache-tools drbd-utils linstor-satellite lsscsi socat thin-send-recv --yes

rm \
    "/buildroot/usr/bin/java" \
    "/buildroot/usr/bin/jexec" \
    "/buildroot/usr/bin/jpackage" \
    "/buildroot/usr/bin/keytool" \
    "/buildroot/usr/bin/rmiregistry"

ARCH="$(dpkg --print-architecture)"

ln -s "/usr/lib/jvm/java-25-openjdk-${ARCH}/lib/jexec" "/buildroot/usr/bin/jexec"
ln -s "/usr/lib/jvm/java-25-openjdk-${ARCH}/bin/java" "/buildroot/usr/bin/java"
ln -s "/usr/lib/jvm/java-25-openjdk-${ARCH}/bin/jpackage" "/buildroot/usr/bin/jpackage"
ln -s "/usr/lib/jvm/java-25-openjdk-${ARCH}/bin/keytool" "/buildroot/usr/bin/keytool"
ln -s "/usr/lib/jvm/java-25-openjdk-${ARCH}/bin/rmiregistry" "/buildroot/usr/bin/rmiregistry"

mv /buildroot/etc/java-25-openjdk/ /buildroot/usr/share/java-25-openjdk/

exit 0
