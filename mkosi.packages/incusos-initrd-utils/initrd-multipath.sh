#!/bin/sh

# Only attempt to activate multipath if there is a duplicate WWN present.

TOTAL_WWNS=$(lsblk -o WWN -dn | grep -v "^$" -c)
TOTAL_UNIQUE_WWNS=$(lsblk -o WWN -dn | grep -v "^$" | sort | uniq | wc -l)

if [ "$TOTAL_WWNS" -ne "$TOTAL_UNIQUE_WWNS" ]; then
    multipath -i

    # Need to sleep a few seconds to allow multipath devices to become available.
    sleep 5
fi
