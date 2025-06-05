# Basic install steps
This provides a brief, high-level overview of how one might install a stand-
alone Incus OS server, add its Incus as a remote, and retrieve the encryption
recovery key.

## Install configuration
First, generate an Incus client certificate/key pair if needed:

    incus remote generate-certificate

Using the [flasher tool](flasher-tool.md), enable the Incus application and
then provide this basic Incus preseed configuration, substituting your local
client certificate (`~/.config/incus/client.crt`):

```
apply_defaults: true
preseed:
    certificates:
        - name: demo
          type: client
          certificate: |
            -----BEGIN CERTIFICATE-----
            MIIB4TCCAWagAwIBAgIQVrBNb+LgEvX/aDNNOLM2iTAKBggqhkjOPQQDAzA4MRkw
            FwYDVQQKExBMaW51eCBDb250YWluZXJzMRswGQYDVQQDDBJnaWJtYXRAZnV0dXJm
            dXNpb24wHhcNMjUwNjA1MTgwODAwWhcNMzUwNjAzMTgwODAwWjA4MRkwFwYDVQQK
            ExBMaW51eCBDb250YWluZXJzMRswGQYDVQQDDBJnaWJtYXRAZnV0dXJmdXNpb24w
            djAQBgcqhkjOPQIBBgUrgQQAIgNiAAS8Tsj87gyhkR6gUoTa9dooWhwApI9MlsZS
            M9HkNdgLG+0d2yU3JXru4AbCD+pslsL5mnSjbmF7BhqSAT0opQtyFMfB7hrCJkVB
            nnebLNOqzrOVnxYqnD1HnfKo6RVmXpGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNV
            HSUEDDAKBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMAoGCCqGSM49BAMDA2kAMGYC
            MQC/Y4nAuV09z/zeh0aN+XV+kI9WLnITFprSHREIaES3r49cTkpoV8wFCwdLjbSb
            NwECMQCx5H/H3hyXJen3uLbqRxTzw5jjx1M4dO4fru+VmoOKmTSmKVq3r2j449iD
            GrzY7EQ=
            -----END CERTIFICATE-----
```

Write out the image and perform the Incus OS installation.

## Add remote Incus OS

After the install completes, you will be shown a list of IP addresses in the
network configuration footer. Pick one and add Incus OS as a remote Incus
server:

```
$ incus remote add IncusOS 10.234.136.23
Certificate fingerprint: 80d569e9244a421f3a3d60d46631eb717f8a0a480f2f23ee729a4c1c016875f7
ok (y/n/[fingerprint])? y

$ incus remote list
+-----------------+------------------------------------+---------------+-------------+--------+--------+--------+
|      NAME       |                URL                 |   PROTOCOL    |  AUTH TYPE  | PUBLIC | STATIC | GLOBAL |
+-----------------+------------------------------------+---------------+-------------+--------+--------+--------+
| IncusOS         | https://10.234.136.23:8443         | incus         | tls         | NO     | NO     | NO     |
+-----------------+------------------------------------+---------------+-------------+--------+--------+--------+
| images          | https://images.linuxcontainers.org | simplestreams | none        | YES    | NO     | NO     |
+-----------------+------------------------------------+---------------+-------------+--------+--------+--------+
| local (current) | unix://                            | incus         | file access | NO     | YES    | NO     |
+-----------------+------------------------------------+---------------+-------------+--------+--------+--------+

```

Once the remote is added, you can interact with it like any other Incus server:

```
$ incus launch images:debian/trixie IncusOS:trixie
Launching trixie

$ incus list
+---------------+---------+------------------------+--------------------------------------------------+-----------------+-----------+
|     NAME      |  STATE  |          IPV4          |                       IPV6                       |      TYPE       | SNAPSHOTS |
+---------------+---------+------------------------+--------------------------------------------------+-----------------+-----------+
| test-incus-os | RUNNING | 10.25.170.1 (incusbr0) | fd42:612d:f700:5f6e::1 (incusbr0)                | VIRTUAL-MACHINE | 0         |
|               |         | 10.234.136.23 (enp5s0) | fd42:3cfb:8972:3990:1266:6aff:feab:9439 (enp5s0) |                 |           |
+---------------+---------+------------------------+--------------------------------------------------+-----------------+-----------+

$ incus list IncusOS:
+--------+---------+----------------------+------------------------------------------------+-----------+-----------+
|  NAME  |  STATE  |         IPV4         |                      IPV6                      |   TYPE    | SNAPSHOTS |
+--------+---------+----------------------+------------------------------------------------+-----------+-----------+
| trixie | RUNNING | 10.25.170.218 (eth0) | fd42:612d:f700:5f6e:1266:6aff:fe39:d31f (eth0) | CONTAINER | 0         |
+--------+---------+----------------------+------------------------------------------------+-----------+-----------+

```

## Fetching the encryption recovery key

Incus OS will warn you if you haven't retrieved the encryption recovery key.
You can do so with the following command. Make sure to store the key someplace
safe!

```
$ incus query IncusOS:/os/1.0/system/encryption
{
        "config": {
                "recovery_keys": [
                        "rteeevbi-iivlfbuc-cuhlldlh-kgbtllru-bhtrulvf-vivlbcgi-dcfuitut-hctjfrvc"
                ]
        },
        "state": {
                "recovery_keys_retrieved": true
        }
}
```
