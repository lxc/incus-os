#!/bin/sh
set -eu

# Record the license information for everything in this image, must run before the documentation is removed.
"${SRCDIR}/incus-osd/generate-licenses" "${SUBIMAGE}" "${BUILDROOT}" "${SRCDIR}/app-build/licenses"
