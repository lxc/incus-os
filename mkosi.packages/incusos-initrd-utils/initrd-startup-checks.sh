#!/bin/sh

# shellcheck disable=SC2028

# Print startup message
for TTY in $TTYS; do
    echo "$OS_NAME is starting..." > "$TTY" || true
done

SECURE_BOOT_DISABLED=false
TPM_MISSING=false

# Check if SecureBoot is enabled
if [ -e /sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c ]; then
    raw_secure_boot_state=$(tail -c 1 /sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c)
    secure_boot_state=$(printf "%d" "'$raw_secure_boot_state")

    if [ "$secure_boot_state" = 1 ]; then
        # Refuse to boot if in Audit Mode; we know of at least one buggy UEFI implementation that marks SecureBoot as enabled while in Audit Mode.
        if [ -e /sys/firmware/efi/efivars/AuditMode-8be4df61-93ca-11d2-aa0d-00e098032b8c ]; then
            raw_audit_mode_state=$(tail -c 1 /sys/firmware/efi/efivars/AuditMode-8be4df61-93ca-11d2-aa0d-00e098032b8c)
            audit_mode_state=$(printf "%d" "'$raw_audit_mode_state")

            if [ "$audit_mode_state" = 1 ]; then
                for TTY in $TTYS; do
                    echo "\033[31m$OS_NAME cannot boot while SecureBoot is in Audit Mode.\033[0m" > "$TTY" || true
                done
                sleep 3600
            fi
        fi
    fi

    if [ "$secure_boot_state" != 1 ]; then
        for TTY in $TTYS; do
            echo "\033[0;33mSecureBoot is disabled. $OS_NAME will attempt to fall back to a less-secure boot logic.\033[0m" > "$TTY" || true
            SECURE_BOOT_DISABLED=true
        done
    fi
else
    for TTY in $TTYS; do
        echo "\033[31mUnable to determine SecureBoot state. $OS_NAME will only boot on UEFI systems.\033[0m" > "$TTY" || true
    done
    sleep 3600
fi

# Check if a v2.0 TPM is present
if [ -e /sys/class/tpm/tpm0/tpm_version_major ]; then
    tpm_version=$(cat /sys/class/tpm/tpm0/tpm_version_major)

    if [ "$tpm_version" != 2 ]; then
        for TTY in $TTYS; do
            echo "\033[31mUnsupported TPM version detected. $OS_NAME requires a v2.0 TPM.\033[0m" > "$TTY" || true
        done
        sleep 3600
    fi
else
    for TTY in $TTYS; do
        echo "\033[0;33mNo TPM detected. $OS_NAME will attempt to fall back to a less-secure swtpm implementation.\033[0m" > "$TTY" || true
        TPM_MISSING=true
    done
fi

# If SecureBoot is disabled and we're missing a TPM, refuse to boot.
if $SECURE_BOOT_DISABLED && $TPM_MISSING; then
    for TTY in $TTYS; do
        echo "\033[31m$OS_NAME cannot boot if SecureBoot is disabled and no physical TPM is present.\033[0m" > "$TTY" || true
    done
    sleep 3600
fi

# If a physical TPM is present, verify that the PE binaries involved in boot (systemd-boot, UKI) match the TPM event log and are properly signed by a trusted certificate.
if ! $TPM_MISSING; then
    peBinaryStatus=$(/usr/bin/incusos-initrd-utils validate-pe-binaries 2>&1)
    if [ "$peBinaryStatus" != "" ]; then
        for TTY in $TTYS; do
            echo "\033[31m$OS_NAME failed to verify PE binaries: $peBinaryStatus\033[0m" > "$TTY" || true
        done
        sleep 3600
    fi
fi
