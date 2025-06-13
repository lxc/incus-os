# Secure Boot details
This document outlines how Incus OS utilizes Secure Boot. A basic understanding of Secure Boot concepts and how a TPM works is assumed.

## Assumptions
  * Prior to installing Incus OS, Secure Boot on the server will be placed into Setup mode. The PK may be populated by the owner; if not present Incus OS will enroll the Incus OS PK CA key during install.
    - Incus OS will completely overwrite any existing KEK keys, and erase all dbx entries.
    - Incus OS will append to any existing db entries.
  * Post-install Secure Boot will run in User mode. This will allow Incus OS to automatically apply db and dbx updates that have been signed by one of the Incus OS KEK certificates. If the Incus OS PK CA key is enrolled, we could also update the KEK keys, although this is expected to be an incredibly rare operation.
    - Alternatively, if Secure Boot is in Deployed mode, db/dbx/KEK key updates will have to be applied out-of-band by IT staff.
  * A new Incus OS Secure Boot Key will be created for each year and published well in advance.
    - Keys will be valid for 18-24 months (TDB) to allow time for rollover. Secure Boot doesn't actually check expiry of certificates, but there is a simple check in `incus-osd` prior to applying an OS update.
    - The first release of Incus OS after January 1st will pickup and use the new year's signing certificate. Some time after that has happened, the prior year's certificate can be invalidated and an update placing that certificate into dbx can be published via API.

## Certificate hierarchy
IncusOS relies on a hierarchy of TLS certificate CAs and certificates as shown below. Note that Secure Boot doesn't perform TLS-style validation of the certificates.

  * IncusOS Root CA and TLS CAs/certificate should use latest standards (ECDSA, etc)
  * IncusOS PK CA and below certificates are limited to RSA 2048 due to the Secure Boot standard

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
  * IncusOS will receive db/dbx/KEK updates via the configured Provider in a similar manner to OS and application updates
  * An update to IncusOS signed with a key not yet in the db or present in dbx will not be allowed to install, since on reboot the system would refuse to boot the new image
  * Updating a trusted signing key and applying an IncusOS update will be two distinct operations, with a reboot in between. This is because updating Secure Boot state and expected TPM PCR values is critical to get right, and it's much simpler logic to only allow one change (a new trusted key or an update with a changed signing key) at a time.

## Secure Boot key updates
KEK, db, and dbx updates are signed offline and then made available via a Provider's API:

  * Published as a simple list of .auth files, with one update per file
  * Filename pattern of `<VAR>_<SHA256 fingerprint>.auth`: db_8A78635EA12B2EF676045B661187E08D1412253220A1BD02EF79D177302DB83F.auth, dbx_DA39EF49E3F5D7B902ECE6CA338883623F61DC671ABE10DF2E7B1CBDEC4A2B47.auth, etc
    - This naming convention makes it trivial for a client to quickly retrieve a list of all available keys, identify any missing ones, and then download needed updates
  * Update check performed on same cadence as OS update checks (every six hours by default):
    - Apply each EFI variable update one at a time, starting with KEK, then db, then dbx
    - Will need to reboot after each update; will automatically reboot if update applied during IncusOS startup, otherwise will require a user triggering a reboot
    - Will not apply a dbx update if the current or backup image are signed by it to prevent bricking

### Update availability and integrity
  * An attacker could block IncusOS update checks to prevent application of Secure Boot key updates
  * Each .auth file is signed by a KEK certificate already enrolled on the machine IncusOS is running on. If the file is tampered with, enrollment will fail, so there is no special need to protect or checksum received updates.

## Use of TPM PCRs
IncusOS relies on two PCRs (7 & 11) to bind disk encryption keys.

### PCR7
PCR7 is computed based on the current Secure Boot state and PK/KEK/db/dbx values. The final value of this PCR is calculated by UEFI before the systemd-boot EFI stub starts. Binding to this PCR allows us to ensure data is only available when Secure Boot is enabled and IncusOS certificates are present. (Prevents an attacker from unlocking the disk on a different machine or launching attacks via live boot media.)

The calculation of PCR7 is straightforward, and performed whenever a signing key is added or revoked, and when an IncusOS update is signed with a different key than the current running system:

  * Fetch TPM event log and verify that the recomputed PCR7 value matches the current TPM PCR7 value
  * Apply EFI variable update
  * Replay TPM event log, computing the future PCR7 value using current EFI variable values
  * Use current TPM state to update PCR7 binding of LUKS volumes using predicted PCR7 value on next boot

### PCR11
PCR11 is computed based on the running UKI, and computed at various points during the boot process. Combined with a properly signed UKI image, this allows us to detect any tampering of the UKI and refuse to unlock the encrypted disks. Computation of PCR11 is complex; systemd has `systemd-measure` which we rely on to create the PCR11 policy which is combined with the Secure Boot signing key to bind the TPM. The advantage is that this approach is much more flexible than an exact hash binding like we do with PCR7, and allows the build process to fully predict PCR11 values and embed those values into the resulting signed UKI images.

IncusOS only ever needs to worry about re-binding PCR11 when the Secure Boot key used by an UKI is changed, such as the yearly key transition. This is because the PCR11 policies are bound to the TPM using the current Secure Boot signing key, and if it changes on reboot the TPM state won't match and auto-unlock will fail. The steps taken when installing an IncusOS update with a different Secure Boot key are:

  * Verify the key of the updated UKI is present in the EFI db variable, and isn't in dbx. This prevents installing an update which will immediately fail to boot with a Secure Boot policy violation.
  * Replace the existing systemd-boot EFI stub with a newly signed one from the pending OS update. `systemd-sysupdate` doesn't typically update the systemd-boot stub, but we need to ensure it's updated to a version signed by the new key.
  * Changing the signature on the systemd-boot stub will affect the PCR7 value at next boot, so follow the steps outlined above to predict the new PCR7 value.
  * Re-bind the TPM PCR11 policies with the new signing certificate and predicted PCR7 value. Doing this invalidates the current TPM state, so we must rely on a recovery key known to IncusOS to update the LUKS header. The update is performed in as an atomic process as possible, to prevent having the LUKS header in a state where it doesn't have a TPM enrolled.

### Implications
Any unexpected change to PCR values will cause auto-unlock to fail, and require the entry of a recovery password to boot the system. When a new Secure Boot key is used, after applying the update and rebooting, attempting to reboot into the backup image will always require the use of the recovery password. Attempting to apply a further OS update while running from the backup image will also very likely fail, since the TPM will be in an unusable state.

### Useful tools
systemd has `systemd-pcrlock` which is useful to inspect current PCR values and how they were computed during the boot process.
  
## Useful links
  * https://uefi.org/specs/UEFI/2.11/32_Secure_Boot_and_Driver_Signing.html#firmware-os-key-exchange-creating-trust-relationships
  * https://uefi.org/specs/UEFI/2.11/32_Secure_Boot_and_Driver_Signing.html#signature-database-update
  * https://techcommunity.microsoft.com/blog/windows-itpro-blog/updating-microsoft-secure-boot-keys/4055324
