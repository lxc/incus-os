# Power

IncusOS supports `reboot`, `poweroff` and `suspend` through the API. IncusOS also supports changing power management configuration such as CPU scaling governor and energy performance preference.

## Actions

### Rebooting IncusOS

IncusOS can be rebooted via

```
incus admin os system reboot
```

By default, IncusOS will boot into the newest installed version of the operating system. The `reboot` and `poweroff` commands support an optional `next_boot_version` parameter that can be set to "newest", "current", or "oldest" to select what version of IncusOS to boot when the system next powers on.

To reboot into the prior installed version of IncusOS automatically, run

```
incus admin os system reboot -d '{"next_boot_version":"oldest"}'
```

### Powering off IncusOS

IncusOS can be safely powered off via

```
incus admin os system poweroff
```

### Suspending IncusOS

IncusOS can be safely suspended via

```
incus admin os system suspend
```

## Configuration options

Power management configuration can be viewed and changed with:

```
incus admin os system power show
incus admin os system power edit
```

Configuration fields are defined in the [`SystemPowerConfig` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_power.go). Changes are applied immediately and re-applied every boot. Removing an option doesn't reset it right away; the current value stays active until the next reboot, after which the kernel default is used.

The following configuration options can be set:

* `cpu`: Change CPU-specific power management configuration.
   * `scaling_governor`: Optional; the CPU frequency scaling governor to use, must be one of the governors supported by the kernel.
   * `energy_performance_preference`: Optional; the CPU energy performance preference to use, must be one of the preferences supported by the kernel. Only available with drivers that support it. When `scaling_governor` is `performance`, this must be unset or set to `performance` as well.

The supported values for the running system can be found with `incus admin os system power show`.

### Examples
Use the `powersave` governor with `balance_power` energy performance preference:

```yaml
config:
  cpu:
    scaling_governor: "powersave"
    energy_performance_preference: "balance_power"
```

Run all the CPUs at full performance:

```yaml
config:
  cpu:
    scaling_governor: "performance"
```
