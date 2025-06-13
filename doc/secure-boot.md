# Secure Boot details
This document outlines how Incus OS utilizes Secure Boot. A basic understanding of Secure Boot concepts and how a TPM works is assumed.

## Certificate hierarchy
IncusOS relies on a hierarchy of TLS certificate CAs and certificates as shown below. Note that Secure Boot doesn't perform TLS-style validation of the certificates.

  * IncusOS Root CA and TLS CAs/certificate should use latest standards (ECDSA, etc)
  * IncusOS PK CA and below certificates are limited to RSA 2048 due to Secure Boot standard

```
                IncusOS Root CA
               /               \
              /                 \
        IncusOS PK CA          (IncusOS TLS CA1)
              |                         |
        IncusOS KEK CA1,CA2             \.........
              |
      /-----------------------\---------------------------------\
     /                         \                                 \
IncusOS Secure Boot Key 2025   IncusOS Secure Boot Key 2026       ......
```

## Use of Secure Boot variables
  * PK: OEM/owner-provided or IncusOS PK CA
  * KEK: IncusOS KEK CA1 & KEK CA2
    - CA2 will be kept offline in cold storage in case CA1 ever becomes unusable
  * db: IncusOS signing key(s)
    - A new signing key will be generated each year and published in advance
    - 6-12 month overlap after January 1st when new signing key is placed into service
  * dbx: Old IncusOS signing key(s)
    - After sufficient time for key rotation, old IncusOS signing keys will be revoked
  * MOK: (not used/supported in IncusOS setup)

### Implementation notes
  * Updates to db and dbx can be signed offline with an enrolled KEK or PK key, then distributed and automatically applied on any system
    - Only replacing existing values or appending are supported
    - Removal of a specific value requires local access to the KEK or PK private key
  * Updates to KEK can be signed offline with an enrolled PK key, and then distributed like db and dbx updates
    - If IncusOS PK CA isn't present, KEK updates will have to be applied out-of-band
    - KEK updates are expected to be extremely rare
  * IncusOS will receive new signing keys and revocation of old keys via some sort of API (details TBD)
  * An update to IncusOS signed with a key not yet trusted by a given system will not be allowed to install, since on reboot the system would refuse to boot the new version
  * Updating a trusted signing key and applying an IncusOS update will be two distinct operations, with a reboot in between. This is because updating Secure Boot state and expected TPM PCR values is critical to get right, and it's much simpler logic to only allow one change (a new trusted key or an update with a changed signing key) at a time.

## Use of TPM PCRs
IncusOS relies on two PCRs (7 & 11) to bind disk encryption keys.

### PCR7
PCR7 is computed based on the current Secure Boot state and PK/KEK/db/dbx values, and the final value is calculated by UEFI before the systemd-boot EFI stub starts. Binding to this PCR allows us to ensure data is only available when Secure Boot is enabled and IncusOS certificates are present. (So an attacker cannot unlock the disk on a different machine or launch attacks via live boot media.)

The calculation of PCR7 is straightforward, and performed whenever a signing key is added or revoked, and when an IncusOS update is signed with a different key than the current running system:

  * Fetch TPM event log and verify recomputed PCR7 value matches current TPM PCR7 value
  * Apply EFI variable update
  * Replay TPM event log, computing future PCR7 value using current EFI variable values
  * Use current TPM state to update PCR7 binding of LUKS volumes using predicted PCR7 value on next boot

### PCR11
PCR11 is computed based on the running UKI, and computed at various points during the boot process. Combined with a properly signed UKI image, this allows us to detect any tampering of the UKI and refuse to unlock the encrypted disks. Computation of PCR11 is complex; systemd has `systemd-measure` which we rely on to create the PCR11 policy which is combined with the Secure Boot signing key to bind the TPM. The advantage is that this approach is much more flexible than an exact hash binding like we do with PCR7, and allows the build process to fully predict PCR11 values and embed those values into the resulting signed UKI images.

IncusOS only ever needs to worry about re-binding PCR11 when the Secure Boot key used by an UKI is changed, such as the yearly key transition. This is because the PCR11 policies are bound to the TPM using the current Secure Boot signing key, and if it changes on reboot the TPM state won't match and auto-unlock will fail. The steps taken when installing an IncusOS update with a different Secure Boot key are:

  * Verify the key of the updated UKI is present in the EFI db variable, and isn't in dbx. The prevents installing an update which will immediately fail to boot with a Secure Boot policy violation.
  * Replace the existing systemd-boot EFI stub with a newly signed one from the pending OS update. The `systemd-sysupdate` doesn't typically update the systemd-boot stub, but we need to ensure it's updated to a version signed by the new key.
  * Changing the signature on the systemd-boot stub will affect the PCR7 value at next boot, so follow a similar set of steps outlined above to predict the new PCR7 value.
  * Re-bind the TPM PCR11 policies with the new signing certificate and predicted PCR7 value. Doing this invalidates the current TPM state, so we must rely on a recovery key known to IncusOS to update the LUKS header. The update is performed in as an atomic process as possible, to prevent having the LUKS header in a state where it doesn't have a TPM enrolled.

### Implications
Any unexpected change to PCR values will cause auto-unlock to fail, and require the entry of a recovery password to boot the system. When a new Secure Boot key is used, after the update and reboot attempting to reboot into the backup image will always require use of the recovery password. Attempting to apply a further OS update while running from the backup image will also very likely fail, since the TPM will be in an unusable state.

### Useful tools
systemd has `systemd-pcrlock` which is useful to inspect current PCR values and how they were computed during the boot process.
  
## Useful links
  * https://uefi.org/specs/UEFI/2.11/32_Secure_Boot_and_Driver_Signing.html#firmware-os-key-exchange-creating-trust-relationships
  * https://uefi.org/specs/UEFI/2.11/32_Secure_Boot_and_Driver_Signing.html#signature-database-update
  * https://techcommunity.microsoft.com/blog/windows-itpro-blog/updating-microsoft-secure-boot-keys/4055324
