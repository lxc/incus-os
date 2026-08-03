# OpenFGA

The OpenFGA (`openfga`) application installs a local OpenFGA service for use by other IncusOS applications, such
as [Operations Center](./operations-center.md).

```{note}
This OpenFGA service is not intended for general-purpose use. By default, it only allows configuring shared
authentication keys, and only the local `sqlite` storage engine is supported.
```

On first start, the `openfga` application will generate a random shared authentication key that can be retrieved
via the application's API as part of its configuration. Additional authentication keys can be set if desired.

The OpenFGA service will automatically use the TLS certificate configured for the primary application to provide a
TLS-secured HTTPS API endpoint on port 8444.

## Synchronization

The `openfga` application also ships [`openfga-sync`](https://github.com/FuturFusion/openfga-sync), a daemon which
pulls group and role information from identity providers (Active Directory/LDAP, Rauthy) and converts it into OpenFGA
relationship tuples on the local stores.

It is configured through the `sync` field of the application configuration, which maps directly to the
`openfga-sync` configuration file. The daemon only runs when a sync configuration is set.

The daemon always synchronizes the local OpenFGA service, its URL and API token are filled in automatically.
The synchronization state is kept on the application's local storage.
