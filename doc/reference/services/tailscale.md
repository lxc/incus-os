# Tailscale

The [Tailscale](https://tailscale.com/) service allows configuring a Tailscale VPN client.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_tailscale.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the Tailscale service.

* `login_server`: The Tailscale login server.

* `auth_key`: A Tailscale authentication key.

* `accept_routes`: If `true`, accept routes.

* `advertised_routes`: An array of routes to advertise.

* `serve_enabled`: If `true`, expose `localhost:8443` (typically the Incus application) via [Tailscale Serve](https://tailscale.com/kb/1242/tailscale-serve)

* `serve_port`: TCP port to expose the HTTPS server to, for example `443` would expose the Incus application on: `https://{hostname}.{tailnet}.ts.net:443/`

```{warning}
Enabling Tailscale Serve requires provisioning HTTPS certificates on the dashboard beforehand ([documentation](https://tailscale.com/kb/1153/enabling-https#configure-https))
```
