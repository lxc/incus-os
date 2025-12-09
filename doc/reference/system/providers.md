# Providers

IncusOS receives [updates](update.md) from the currently configured provider. Two providers are supported:

* `images`: The default IncusOS provider, which fetches updates from the [Linux Containers {abbr}`CDN (Content Delivery Network)`](https://images.linuxcontainers.org/os/).

* `operations-center`: When IncusOS is deployed in a managed environment controlled by [Operations Center](../applications/operations-center.md), it is registered with the `operations-center` provider. This allows an administrator to centrally control all IncusOS systems, even in restricted or air-gaped environments that may not have Internet access.

## Configuration options

Configuration fields are defined in the [`SystemProviderConfig` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_provider.go).

The following configuration options can be set:

* `name`: The name of the provider. One of `images`, `operations-center`, or `local`. `local` is intended for use by developers working on IncusOS.

* `config`: A map of provider-specific configuration key-value pairs.
