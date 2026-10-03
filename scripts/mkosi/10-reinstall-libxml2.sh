#!/bin/sh -eu

[ "$1" = "final" ] || exit 0

# Reinstall libxml2 (dropped from base) before application packages need it.
mkosi-reinstall libxml2
