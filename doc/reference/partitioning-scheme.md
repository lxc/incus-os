# Partitioning scheme

IncusOS utilizes `systemd-repart` to automatically partition the main system drive at first boot. The layout of the partition table looks like the following:

    EFI ESP (2GiB)
    seed data (100MiB)
    A-side root partition signing (16kiB)
    A-side root partition hashes (100MiB)
    A-side root partition (1GiB)
    B-side root partition signing (16KiB)
    B-side root partition hashes (100MiB)
    B-side root partition (1GiB)
    LUKS encrypted swap (4GiB)
    LUKS encrypted ext4 system data (25 GiB)
    ZFS encrypted pool "local" (remaining space)

## A/B updates

The EFI ESP partition holds two different signed UKI images, each corresponding to an A- or B-side root partition. When an OS update is applied, the non-booted UKI is replaced and its corresponding signing, hash, and data partitions are atomically updated. On reboot, `systemd-boot` will automatically select the updated UKI.

## Seed data partition

The seed data partition is used during install or [factory reset](system/backup.md).

## Encrypted partitions

Partitions that hold user data are encrypted. The swap and ext4 system partitions are both encrypted and under normal operation are automatically unlocked during boot by the TPM. If unlocking fails for some reason, a recovery key can be provided to allow the system to boot.

Each ZFS pool created by IncusOS is encrypted with a randomly generated key. These keys are stored in the encrypted system partition.

The encryption keys can be retrieved via the [security API](system/security.md).

### System partition

The system partition holds any system data that is not part of the immutable IncusOS images.

### "local" ZFS pool

The "local" ZFS pool consumes all remaining space on the main system drive. It is available for use by applications; for example, when Incus is installed it will create a dataset `local/incus` to use as the default storage pool for containers and virtual machines.
