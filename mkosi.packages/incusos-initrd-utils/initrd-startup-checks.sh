#!/bin/sh

# shellcheck disable=SC2028

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
