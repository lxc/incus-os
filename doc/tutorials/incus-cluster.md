# Creating an Incus cluster

Setting up an Incus cluster with IncusOS is a bit different than on regular Linux systems.

That's because most Incus cluster initialization and then growth is
typically handled through `incus admin init`. That command is only
available directly on the system running Incus, which in the case of
IncusOS makes it inaccessible.

Instead on IncusOS, one needs to get the Incus servers up and running,
then add them as remotes and finally use the `incus cluster enable` and
`incus cluster join` commands to assemble the cluster.

## Initializing a new cluster
The first step is to get the first of the systems up and running with IncusOS.

For that, follow the [normal installation
instructions](../getting-started/installation.md), making sure to get an
image with the default Incus settings enabled (`Apply default
configuration` in the web-based image customizer).

Once the IncusOS system is up and running, add it as a remote to your
Incus command line tool using `incus remote add`.

At that point, you're ready to make it a cluster with:

```
incus config set server1: cluster.https_address=10.0.0.10:8443
incus cluster enable server1: server1

incus remote add my-cluster 10.0.0.10:8443
incus remote remove server1
```

That will set the IP address for all internal cluster communications to
`10.0.0.10`, then enable clustering, setting the initial server name to
`server1`, then add a new Incus remote for the cluster named
`my-cluster` and lastly remove the old server remote.

## Adding additional servers
Additional servers need to use an installation image WITHOUT the default
Incus settings enabled (`Apply default configuration` in the web-based image
customizer).

That's important as joining servers cannot have preexisting networks or
storage pools defined, both of which get created as part of the default
configuration.

Once the server is installed and added as a remote to the Incus command line tool
using `incus remote add`, it can be added to the cluster with:

```
incus cluster join my-cluster: server2:
incus remote remove server2
incus cluster list my-cluster:
```

This will get `server2` to join `my-cluster`, then remove the
server-specific `server2` remote and show an overview of the servers in
the cluster.

```{note}
Use `local/incus` as the value for the `source` and `zfs.pool_name` keys on the `local` storage pool.
```
