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

* `serve_service`: Tailscale [Service](https://tailscale.com/docs/features/tailscale-services) to expose the service as. Corresponds to the `--service=` flag in `tailscale serve`, not passed if empty. Note: At time of writing this feature is [not supported in Headscale](https://github.com/juanfont/headscale/issues/2845).

* `advertise_exit_node`: If `true`, offer to be an [exit node](https://tailscale.com/docs/features/exit-nodes#advertise-a-device-as-an-exit-node) for internet traffic for the tailnet.

* `exit_node`: Tailscale exit node (IP, base name, or auto:any) for internet traffic, or empty string to not use an exit node.

* `exit_node_allow_lan_access`: If `true`, allow direct access to the local network when routing traffic via an exit node. 

```{note}
Enabling Tailscale Serve requires provisioning HTTPS certificates on the dashboard beforehand ([documentation](https://tailscale.com/kb/1153/enabling-https#configure-https))
```

```{warning}
Setting `serve_port` to `8443` without changing the Incus listen address can cause a port conflict on boot preventing any further connection to the system.
```
