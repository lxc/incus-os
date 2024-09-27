#!/bin/sh
[ "$1" = "final" ] || exit 0

# Get the repository keyring key.
if [ ! -e /etc/apt/keyrings/zabbly.asc ]; then
    mkdir -p /etc/apt/keyrings/
    curl -fsSL https://pkgs.zabbly.com/key.asc -o /etc/apt/keyrings/zabbly.asc
fi

# Add the repository.
cat <<EOF > /etc/apt/sources.list.d/zabbly-incus-daily.sources
Enabled: yes
Types: deb
URIs: https://pkgs.zabbly.com/incus/daily
Suites: bookworm
Components: main
Architectures: amd64
Signed-By: /etc/apt/keyrings/zabbly.asc

EOF

# Install the incus packages.
apt-get update
apt-get install incus incus-ui-canonical --yes

# Mark everything left as manually installed.
dpkg -l | grep ^ii | awk '{print $2}' | xargs apt-mark manual

# Then remove incus itself.
apt-get remove --purge incus incus-base incus-client incus-ui-canonical --yes

exit 0
