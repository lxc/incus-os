#!/bin/sh -eux

# Run this from a build script so it runs before the final depmod run.
rm -Rf \
    "${DESTDIR}/usr/lib/modules/*/vmlinuz" \
    "${DESTDIR}/usr/lib/modules/*/kernel/sound" \
    "${DESTDIR}/usr/lib/modules/*/kernel/drivers/gpu" \
    "${DESTDIR}/usr/lib/modules/*/kernel/drivers/infiniband" \
    "${DESTDIR}/usr/lib/modules/*/kernel/drivers/iio" \
    "${DESTDIR}/usr/lib/modules/*/kernel/drivers/media" \
    "${DESTDIR}/usr/lib/modules/*/kernel/drivers/net/wireless"

exit 0
