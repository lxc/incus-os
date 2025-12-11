#!/bin/sh

# shellcheck disable=SC2028

# Print startup message
for TTY in $TTYS; do
    echo "$OS_NAME is starting..." > "$TTY" || true
done

# Check if SecureBoot is enabled
if [ -e /sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c ]; then
    raw_secure_boot_state=$(tail -c 1 /sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c)
    secure_boot_state=$(printf "%d" "'$raw_secure_boot_state")

    if [ "$secure_boot_state" != 1 ]; then
        for TTY in $TTYS; do
            echo "\033[31mSecureBoot is disabled. $OS_NAME cannot start until SecureBoot is enabled.\033[0m" > "$TTY" || true
        done
        sleep 3600
    fi
else
    for TTY in $TTYS; do
        echo "\033[31mUnable to determine SecureBoot state. $OS_NAME cannot start until SecureBoot is enabled.\033[0m" > "$TTY" || true
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
    done
fi
