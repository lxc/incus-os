#!/bin/sh

# shellcheck disable=SC2028

# Unlock the root LUKS volume using a recovery key found on a RESCUE_DATA partition.
# The key is securely destroyed first and only used if that succeeded.

umask 077

MOUNT_DIR="/run/incus-os-recovery"
KEY_FILE="${MOUNT_DIR}/recovery.txt"
TMP_KEY="/run/incus-os-recovery.key"

# Find the recovery partition.
DEVICE=""
for CANDIDATE in /dev/disk/by-partlabel/RESCUE_DATA /dev/disk/by-label/RESCUE_DATA; do
    if [ -e "${CANDIDATE}" ]; then
        DEVICE="${CANDIDATE}"
        break
    fi
done

if [ -z "${DEVICE}" ]; then
    exit 0
fi

# Only FAT filesystems are supported, as the key must be destroyed after use.
mkdir -p "${MOUNT_DIR}"
if ! mount -t vfat -o rw "${DEVICE}" "${MOUNT_DIR}"; then
    rmdir "${MOUNT_DIR}"
    exit 0
fi

cleanup() {
    rm -f "${TMP_KEY}"
    umount "${MOUNT_DIR}" 2> /dev/null || true
    rmdir "${MOUNT_DIR}" 2> /dev/null || true
}
trap cleanup EXIT

if [ ! -f "${KEY_FILE}" ]; then
    exit 0
fi

# Give the user a chance to abort before the key is destroyed.
for TTY in $TTYS; do
    echo "\033[0;33mA recovery key was found on the RESCUE_DATA partition. It will be used to unlock $NAME in 60 seconds, and then securely destroyed. Power off now to abort.\033[0m" > "$TTY" || true
done
sleep 60

# Read the recovery key, stripping any line endings.
if ! tr -d '\r\n' < "${KEY_FILE}" > "${TMP_KEY}"; then
    exit 1
fi

# Securely destroy the key before attempting to use it.
if ! shred -u "${KEY_FILE}" || ! sync || ! umount "${MOUNT_DIR}"; then
    for TTY in $TTYS; do
        echo "\033[31mFailed to securely destroy the recovery key, refusing to use it.\033[0m" > "$TTY" || true
    done
    exit 1
fi

# Unlock the root volume.
if ! systemd-cryptsetup attach root /dev/gpt-auto-root-luks "${TMP_KEY}" tpm2-measure-pcr=yes,tries=1; then
    for TTY in $TTYS; do
        echo "\033[31mFailed to unlock $NAME with the recovery key.\033[0m" > "$TTY" || true
    done
    exit 1
fi

# Record that a recovery key was used to unlock the volume.
touch /run/incus-os-recovery-key-used
