# Fallback listener

The IncusOS API is normally reached through the primary application. The fallback listener is a separate HTTPS endpoint run by IncusOS itself, which keeps the IncusOS API reachable when the primary application is unavailable.

The fallback listener is automatically started when:

* IncusOS boots from the prior installed version.
* The primary application fails to start or to reload after an update.
* The `persistent` configuration option is set.

Once started, the fallback listener keeps running until the next reboot. It uses the same server certificate as the primary application and only accepts clients presenting one of its trusted client certificates.

See [System recovery](../recovery.md) for how the fallback listener fits in the overall recovery process.

## Configuration options

Configuration fields are defined in the [`SystemFallbackListenerConfig` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_fallback_listener.go).

The following configuration options can be set:

* `listen_address`: Optional; the address to listen on, either as an `IP:port` such as `10.0.0.1:8445` or as a `:port` such as `:8445` to listen on all interfaces. When unset, IncusOS listens on all interfaces on a random port, which is shown on the system's terminal and in its logs. A change only applies the next time the fallback listener is started.

* `persistent`: If `true`, always start the fallback listener at boot rather than only when the primary application is unavailable. Enabling the option also starts the fallback listener right away.

* `trusted_client_certificates`: A list of PEM-encoded client certificates allowed to connect to the fallback listener. At least one certificate must be defined. Changes apply immediately.

On first boot, IncusOS sets the trusted client certificates from those found in the seed data of the primary application. When registering with [Operations Center](../applications/operations-center.md), its client certificate is added as well. No other synchronization with the primary application is performed, so later changes to its trusted clients must be manually applied to the fallback listener.

## State

The following state is reported alongside the configuration:

* `active`: Whether the fallback listener is currently running.

## Examples

Show the current configuration and state:

```
incus admin os system fallback-listener show
```

Edit the configuration, for example to make the fallback listener persistent on a fixed port:

```
incus admin os system fallback-listener edit
```

```
config:
  listen_address: :8445
  persistent: true
  trusted_client_certificates:
  - |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```
