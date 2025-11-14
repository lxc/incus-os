#!/bin/sh
KERNEL="$(ls /buildroot/usr/lib/modules/)"

# Install the drbd sources
mkdir -p /run/lock
apt-get install --no-install-recommends --yes drbd-dkms || true

# Patch the module source
cp "/buildroot/boot/config-${KERNEL}" "/buildroot/usr/src/linux-headers-${KERNEL}/.config"
cd /buildroot/usr/src/drbd-*/src/ || exit 1
(
cat << EOF
ZGlmZiAtLWdpdCBhL2RyYmQvS2J1aWxkLmRyYmQgYi9kcmJkL0tidWlsZC5kcmJkCmluZGV4IDU5
MGQ2ZDJmMi4uODU3ZDg5ZjYxIDEwMDY0NAotLS0gYS9kcmJkL0tidWlsZC5kcmJkCisrKyBiL2Ry
YmQvS2J1aWxkLmRyYmQKQEAgLTQ5LDEzICs0OSw4IEBAIG9iai0kKGlmICQoQ09ORklHX0lORklO
SUJBTkQpLG0pICs9IGRyYmRfdHJhbnNwb3J0X3JkbWEubwogb2JqLSQoQ09NUEFUX05FVF9IQU5E
U0hBS0UpICAgICAgKz0gZHJiZC1rZXJuZWwtY29tcGF0L2hhbmRzaGFrZS8KICMgPT09PT09PT09
PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PQog
Ci1pZmRlZiBDT05GSUdfREVWX0RBWF9QTUVNCi1pZm5lcSAoJChzaGVsbCBncmVwIC1lICdcPGFy
Y2hfd2JfY2FjaGVfcG1lbVw+JyAkKG9ianRyZWUpL01vZHVsZS5zeW12ZXJzIHwgd2MgLWwpLDEp
Ci1vdmVycmlkZSBFWFRSQV9DRkxBR1MgKz0gLUREQVhfUE1FTV9JU19JTkNPTVBMRVRFCi1lbHNl
Ci1DT05GSUdfRFJCRF9EQVggOj0geQotZW5kaWYKLWVuZGlmCitDT05GSUdfREVWX0RBWF9QTUVN
IDo9IG4KK0NPTkZJR19EUkJEX0RBWCA6PSBuCiAKICMgZW5hYmxlIGZhdWx0IGluamVjdGlvbiBi
eSBkZWZhdWx0CiBpZm5kZWYgQ09ORklHX0RSQkRfRkFVTFRfSU5KRUNUSU9OCmRpZmYgLS1naXQg
YS9kcmJkL2RyYmRfYWN0bG9nLmMgYi9kcmJkL2RyYmRfYWN0bG9nLmMKaW5kZXggM2M3ZjEzM2Ix
Li45NWI4NWRlN2MgMTAwNjQ0Ci0tLSBhL2RyYmQvZHJiZF9hY3Rsb2cuYworKysgYi9kcmJkL2Ry
YmRfYWN0bG9nLmMKQEAgLTIwMyw3ICsyMDMsNyBAQCBzdHJ1Y3QgbGNfZWxlbWVudCAqX2FsX2dl
dF9ub25ibG9jayhzdHJ1Y3QgZHJiZF9kZXZpY2UgKmRldmljZSwgdW5zaWduZWQgaW50IGVucgog
CXJldHVybiBhbF9leHQ7CiB9CiAKLSNpZiBJU19FTkFCTEVEKENPTkZJR19ERVZfREFYX1BNRU0p
ICYmICFkZWZpbmVkKERBWF9QTUVNX0lTX0lOQ09NUExFVEUpCisjaWYgMAogc3RhdGljCiBzdHJ1
Y3QgbGNfZWxlbWVudCAqX2FsX2dldChzdHJ1Y3QgZHJiZF9kZXZpY2UgKmRldmljZSwgdW5zaWdu
ZWQgaW50IGVucikKIHsKZGlmZiAtLWdpdCBhL2RyYmQvZHJiZF9kYXhfcG1lbS5oIGIvZHJiZC9k
cmJkX2RheF9wbWVtLmgKaW5kZXggOTkyY2IyY2ExLi5lNTEwYjA1NzQgMTAwNjQ0Ci0tLSBhL2Ry
YmQvZHJiZF9kYXhfcG1lbS5oCisrKyBiL2RyYmQvZHJiZF9kYXhfcG1lbS5oCkBAIC00LDcgKzQs
NyBAQAogCiAjaW5jbHVkZSA8bGludXgva2NvbmZpZy5oPgogCi0jaWYgSVNfRU5BQkxFRChDT05G
SUdfREVWX0RBWF9QTUVNKSAmJiAhZGVmaW5lZChEQVhfUE1FTV9JU19JTkNPTVBMRVRFKQorI2lm
IDAKIAogaW50IGRyYmRfZGF4X29wZW4oc3RydWN0IGRyYmRfYmFja2luZ19kZXYgKmJkZXYpOwog
dm9pZCBkcmJkX2RheF9jbG9zZShzdHJ1Y3QgZHJiZF9iYWNraW5nX2RldiAqYmRldik7Cg==
EOF
) | base64 -d | patch -p1

# Build the patched module
apt-get install --no-install-recommends --yes drbd-dkms

# Sign the module
mkdir -p "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/"

for mod in drbd drbd_transport_tcp drbd_transport_lb-tcp drbd_transport_rdma; do
    "/buildroot/usr/src/linux-headers-${KERNEL}/scripts/sign-file" \
        sha256 /work/src/mkosi.key /work/src/mkosi.crt \
        "/buildroot/usr/lib/modules/${KERNEL}/updates/dkms/${mod}.ko" \
        "${DESTDIR}/usr/lib/modules/${KERNEL}/updates/dkms/${mod}.ko"
done
