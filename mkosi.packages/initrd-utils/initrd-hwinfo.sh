#!/bin/sh

# shellcheck disable=SC2028,SC2086

while true; do
    for TTY in $TTYS; do
        echo "\033cDisplaying storage devices detected by $NAME:" > "$TTY" || true
        lsblk -d -o NAME,SIZE,MODEL,SERIAL > "$TTY" || true
    done
    sleep 10

    for TTY in $TTYS; do
        echo "\033cDisplaying network devices detected by $NAME:" > "$TTY" || true
        lshw -class network | grep -E "product:|name:|serial:|size:| \*-network" > "$TTY" || true
    done
    sleep 10
done
