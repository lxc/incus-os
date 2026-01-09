#!/bin/sh

# shellcheck disable=SC2028,SC2086

while true; do
    for TTY in $TTYS; do
        echo "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ systemctl --failed" > "$TTY" || true
        systemctl --failed > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ journalctl -b 0 --priority 3" > "$TTY" || true
        journalctl -b 0 --priority 3 > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ lsblk" > "$TTY" || true
        lsblk > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ lscpi" > "$TTY" || true
        lspci > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ lsusb" > "$TTY" || true
        lsusb > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ ls -lh /sys/class/block/*/device/driver" > "$TTY" || true
        ls -lh /sys/class/block/*/device/driver > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ dmesg | grep -i tpm" > "$TTY" || true
        dmesg | grep -i tpm > "$TTY" || true
        echo "$ ls -lh /dev/tpm*" > "$TTY" || true
        ls -lh /dev/tpm* > "$TTY" || true
        echo "$ for i in /sys/class/tpm/*/tpm_version_major; do echo \"\$i => \$(cat $i)\"; done" > "$TTY" || true
        for i in /sys/class/tpm/*/tpm_version_major; do
            echo "$i => $(cat $i)" > "$TTY" || true;
        done
    done
    sleep 10
done
