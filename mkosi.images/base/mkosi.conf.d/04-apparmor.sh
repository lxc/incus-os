#!/bin/sh -eux

# Copy apparmor configuration to /usr/share/.
mkdir -p "${DESTDIR}/usr/share/"
cp -r /buildroot/etc/apparmor/ "${DESTDIR}/usr/share/"
cp -r /buildroot/etc/apparmor.d/ "${DESTDIR}/usr/share/"

# Only keep the shared includes, none of the packaged profiles apply to IncusOS.
for entry in "${DESTDIR}/usr/share/apparmor.d/"*; do
    case "$(basename "${entry}")" in
        abi|abstractions|tunables)
            ;;
        *)
            rm -rf "${entry}"
            ;;
    esac
done

exit 0
