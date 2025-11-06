# Applications
IncusOS itself doesn't listen on the network. For the system to be
accessible and manageable over the network, it requires a primary
application to be installed.

The primary application is responsible for listening on the network and
for handling user authentication. It then provides access to the IncusOS
management API through its own API.

IncusOS also supports additional (non-primary) applications which can
extend the base system (for example for debugging) or provide additional
features to another application.

```{toctree}
:maxdepth: 1

Incus </reference/applications/incus>
Migration Manager </reference/applications/migration-manager>
Operations Center </reference/applications/operations-center>

Shared API </reference/applications/shared-api>
```
