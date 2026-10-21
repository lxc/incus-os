# Power

By default, IncusOS will boot into the newest installed version of the operating system. The `reboot` and `poweroff` commands support an optional `next_boot_version` parameter that can be set to "newest", "current", or "oldest" to select what version of IncusOS to boot when the system next powers on.

## Rebooting IncusOS

IncusOS can be rebooted via

```
incus admin os system reboot
```

To reboot into the prior installed version of IncusOS automatically, run

```
incus admin os system reboot -d '{"next_boot_version":"oldest"}'
```

## Powering off IncusOS

IncusOS can be safely powered off via

```
incus admin os system poweroff
```

## Suspending IncusOS

IncusOS can be safely suspended via

```
incus admin os system suspend
```
