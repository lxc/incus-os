#!/bin/sh

# shellcheck disable=SC3000-SC4000

while true; do
    for TTY in $TTYS; do
        echo -e "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ lsblk" > "$TTY" || true
        lsblk > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo -e "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ lscpi" > "$TTY" || true
        lspci > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo -e "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ lsusb" > "$TTY" || true
        lsusb > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo -e "\033cIncusOS failed to start. Debug information follows." > "$TTY" || true
        echo "$ ls -lh /sys/class/block/*/device/driver" > "$TTY" || true
        ls -lh /sys/class/block/*/device/driver > "$TTY" || true
    done
    sleep 10
done
