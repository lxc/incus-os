# Security

IncusOS has a fairly robust [security setup](../security.md) that is enforced on all systems. Under normal operation, IncusOS relies on the TPM to automatically unlock the main system drive, which in turn holds the encryption keys for any [storage pools](storage.md).

As part of its first boot, IncusOS generates a strong recovery key that can be used to decrypt the main system drive in recovery scenarios, such accidental TPM reset or needing to perform offline data recovery. Additional recovery keys can be added if desired. It is imperative to protect the recovery key(s) in a manner consistent with the importance of data stored on the corresponding IncusOS system.

The recovery key(s) can be retrieved by running

```
incus admin os system security show
```

## Configuration options

The following configuration options can be set:

* `encryption_recovery_keys`: An array of one or more encryption recovery keys for the IncusOS main system drive. At least one recovery key must always be provided, but no length or complexity policy is enforced by IncusOS. Any existing recovery key(s) not present in the array will be removed, and any new key(s) will be added.

## Resetting TPM bindings

If IncusOS fails to automatically unlock the main system drive, after booing using a recovery key, it is possible to forcefully reset the TPM bindings:

```
incus admin os system security tpm-rebind
```
