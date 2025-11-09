#!/bin/sh
[ "$1" = "final" ] || exit 0

# Get the repository keyring key.
if [ ! -e /etc/apt/keyrings/zabbly.asc ]; then
    mkdir -p /etc/apt/keyrings/
    curl -fsSL https://pkgs.zabbly.com/key.asc -o /etc/apt/keyrings/zabbly.asc
fi

# Add the repository.
cat <<EOF > /etc/apt/sources.list.d/zabbly-incus-stable.sources
Enabled: yes
Types: deb
URIs: https://pkgs.zabbly.com/incus/stable
Suites: trixie
Components: main
Signed-By: /etc/apt/keyrings/zabbly.asc

EOF

# Install the incus packages.
mkdir -p /run/lock
apt-get update
apt-get install ceph-common --yes
apt-get install drbd-utils linstor-satellite lsscsi socat thin-send-recv --yes
apt-get install incus incus-ui-canonical --yes

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
