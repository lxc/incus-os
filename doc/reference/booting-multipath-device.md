# Booting from a multipath-backed device

IncusOS supports booting from a multipath-backed boot device. During early boot, IncusOS will automatically assemble detected devices with identical {abbr}`WWN (World Wide Name)`s into a multipath device. Once IncusOS is booted, the [multipath](./services/multipath.md) service can be used to further configure the multipath device.
