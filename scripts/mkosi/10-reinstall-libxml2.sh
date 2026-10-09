#!/bin/sh -eu

[ "$1" = "final" ] || exit 0

# Reinstall libxml2 (dropped from base) before application packages need it, remove after January 2027.
mkosi-reinstall libxml2
