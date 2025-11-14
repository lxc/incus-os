#!/bin/sh
[ "$1" = "final" ] || exit 0

# Install the packages.
mkdir -p /run/lock
apt-get update
apt-get install drbd-utils linstor-satellite lsscsi socat thin-send-recv --yes

rm \
    "/buildroot/usr/bin/java" \
    "/buildroot/usr/bin/jexec" \
    "/buildroot/usr/bin/jpackage" \
    "/buildroot/usr/bin/keytool" \
    "/buildroot/usr/bin/rmiregistry"

ln -s "/usr/lib/jvm/java-21-openjdk-amd64/lib/jexec" "/buildroot/usr/bin/jexec"
ln -s "/usr/lib/jvm/java-21-openjdk-amd64/bin/java" "/buildroot/usr/bin/java"
ln -s "/usr/lib/jvm/java-21-openjdk-amd64/bin/jpackage" "/buildroot/usr/bin/jpackage"
ln -s "/usr/lib/jvm/java-21-openjdk-amd64/bin/keytool" "/buildroot/usr/bin/keytool"
ln -s "/usr/lib/jvm/java-21-openjdk-amd64/bin/rmiregistry" "/buildroot/usr/bin/rmiregistry"

mv /buildroot/etc/java-21-openjdk/ /buildroot/usr/share/java-21-openjdk/

exit 0
