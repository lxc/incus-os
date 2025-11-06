# Accessing the system

```{note}
These instructions assume IncusOS deployed with the Incus application.

When using it with Operations Center or Migration Manager, use their respective command line client or web UI instead.
```

## From the command line
After the install completes, you will be shown a list of IP addresses in the
network configuration footer. Pick one and add IncusOS as a remote Incus
server:

```
$ incus remote add IncusOS 192.0.2.100
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

```{note}
192.0.2.100 is used here as the example IP address of the IncusOS system.
The current IP address can be found at the bottom of the screen on the running system.
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

## From the web
The Incus UI is also available for web access.

For this to work, the client certificate provided at image creation time
must be imported as a user certificate in your web browser.

The exact process to do this varies between browsers and operating
systems, but generally involves generating a PKCS#12 certificate from
the separate `client.crt` and `client.key`, then importing that in the
web browser's certificate store.

Once this is done, you can access the UI at `https://192.0.2.100:8443`

```{note}
192.0.2.100 is used here as the example IP address of the IncusOS system.
The current IP address can be found at the bottom of the screen on the running system.
```

## Fetching the encryption recovery key

IncusOS will warn you if you haven't retrieved the encryption recovery key.
You can do so with the following command. Make sure to store the key someplace
safe!

```{note}
This step is currently only possible through the command line client.
```

```
$ incus query IncusOS:/os/1.0/system/security
{
        "config": {
                "encryption_recovery_keys": [
                        "fkrjjenn-tbtjbjgh-jtvvchjr-ctienevu-crknfkvi-vjlvblhl-kbneribu-htjtldch"
                ]
        },
        "state": {
                "encrypted_volumes": [
                        {
                                "state": "unlocked (TPM)",
                                "volume": "root"
                        },
                        {
                                "state": "unlocked (TPM)",
                                "volume": "swap"
                        }
                ],
                "encryption_recovery_keys_retrieved": true,
                "pool_recovery_keys": {
                        "local": "F7zrtdHEaivKqofZbVFs2EeANyK77DbLi6Z8sqYVhr0="
                },
                "secure_boot_certificates": [
                        {
                                "fingerprint": "26dce4dbb3de2d72bd16ae91a85cfeda84535317d3ee77e0d4b2d65e714cf111",
                                "issuer": "CN=Incus OS - Secure Boot E1,O=Linux Containers",
                                "subject": "CN=Incus OS - Secure Boot PK R1,O=Linux Containers",
                                "type": "PK"
                        },
                        {
                                "fingerprint": "9a42866f496834bde7e1b26a862b1e1b6dea7b78b91a948aecfc4e6ef79ea6c1",
                                "issuer": "CN=Incus OS - Secure Boot E1,O=Linux Containers",
                                "subject": "CN=Incus OS - Secure Boot KEK R1,O=Linux Containers",
                                "type": "KEK"
                        },
                        {
                                "fingerprint": "21b6f423cf80fe6c436dfea0683460312f276debe2a14285bfdc22da2d00fc20",
                                "issuer": "CN=Incus OS - Secure Boot E1,O=Linux Containers",
                                "subject": "CN=Incus OS - Secure Boot 2025 R1,O=Linux Containers",
                                "type": "db"
                        }
                ],
                "secure_boot_enabled": true,
                "tpm_status": "ok"
        }
}
```
