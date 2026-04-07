package systemd

var (
	// SystemExtensionsPath is the systemd location for system extensions.
	SystemExtensionsPath = "/var/lib/extensions"

	// LocalExtensionsPath is the location where IncusOS stores the actual system extensions.
	LocalExtensionsPath = "/var/lib/incus-os-extensions"

	// SystemUpdatesPath is the systemd location for system updates.
	SystemUpdatesPath = "/var/lib/updates"

	// SystemdNetworkConfigPath is the location for systemd network config files.
	SystemdNetworkConfigPath = "/run/systemd/network/"

	// SystemdTimesyncConfigFile is the configuration file for systemd-timesyncd.
	SystemdTimesyncConfigFile = "/run/systemd/timesyncd.conf"
)
