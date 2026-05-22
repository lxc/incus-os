# OpenFGA

The OpenFGA (`openfga`) application installs a local OpenFGA service for use by other IncusOS applications, such
as [Operations Center](./operations-center.md).

```{note}
This OpenFGA service is not intended for general-purpose use. It is unavailable over the network (listening only
on `localhost`) and only allows configuring shared authentication keys.
```

On first start, the `openfga` application will generate a random authentication key that can be retrieved via the
application's API as part of its configuration. Additional authentication keys can be set if desired.

A default OpenFGA store will also be created and its ID recorded in the application's state.
