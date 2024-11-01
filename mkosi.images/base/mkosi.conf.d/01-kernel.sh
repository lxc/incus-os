#!/bin/sh -eux
[ "$1" = "final" ] || exit 0

# Get the repository keyring key.
if [ ! -e /etc/apt/keyrings/zabbly.asc ]; then
    mkdir -p /etc/apt/keyrings/
    curl -fsSL https://pkgs.zabbly.com/key.asc -o /etc/apt/keyrings/zabbly.asc
fi

# Add the repository.
cat <<EOF > /etc/apt/sources.list.d/zabbly-kernel-stable.sources
Enabled: yes
Types: deb
URIs: https://pkgs.zabbly.com/kernel/stable
Suites: bookworm
Components: main
Architectures: amd64
Signed-By: /etc/apt/keyrings/zabbly.asc

EOF

# Install the kernel.
apt-get update
apt-get install linux-zabbly --yes

exit 0
