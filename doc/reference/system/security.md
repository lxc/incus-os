# Security

IncusOS has a fairly robust [security setup](../security.md) that is enforced on all systems. Under normal operation, IncusOS relies on the TPM to automatically unlock the main system drive, which in turn holds the encryption keys for any [storage pools](storage.md).

As part of its first boot, IncusOS generates a strong recovery key that can be used to decrypt the main system drive in recovery scenarios, such accidental TPM reset or needing to perform offline data recovery. Additional recovery keys can be added if desired. It is imperative to protect the recovery key(s) in a manner consistent with the importance of data stored on the corresponding IncusOS system.

The recovery key(s) can be retrieved by running

```
incus admin os system security show
```

## Configuration options

Configuration fields are defined in the [`SystemSecurityConfig` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_security.go).

The following configuration options can be set:

* `encryption_recovery_keys`: An array of one or more encryption recovery keys for the IncusOS main system drive. At least one recovery key must always be provided. Any existing recovery key(s) not present in the array will be removed, and any new key(s) will be added. A very simple complexity policy is enforced by IncusOS:
   * At least 15 characters long
   * Contain at least one special character
   * Consist of at least five unique characters
   * Some other simple complexity checks are applied, and any encryption recovery key that doesn't pass will be rejected with an error

## Resetting TPM bindings

If IncusOS fails to automatically unlock the main system drive, after booting using a recovery key, it is possible to forcefully reset the TPM bindings:

```
incus admin os system security tpm-rebind
```
