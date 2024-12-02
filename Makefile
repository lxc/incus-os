.PHONY: default
default: build

ifeq (, $(shell which mkosi))
$(error "mkosi couldn't be found, please install it and try again")
endif

.PHONY: clean
clean:
	sudo -E rm -Rf .cache/ mkosi.output/
	sudo -E $(shell command -v mkosi) clean

.PHONY: build
build:
	-mkosi genkey
	mkdir -p mkosi.images/base/mkosi.extra/boot/EFI/
	openssl x509 -in mkosi.crt -out mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der -outform DER
	sudo rm -Rf mkosi.output/base* mkosi.output/debug* mkosi.output/incus*
	sudo -E $(shell command -v mkosi) --cache-dir .cache/ build
	sudo chown $(shell id -u):$(shell id -g) mkosi.output

.PHONY: test
test:
	incus delete -f test-incus-os || true
	incus image delete incus-os || true

	qemu-img convert -f raw -O qcow2 $(shell ls mkosi.output/IncusOS_*.raw | grep -v usr | grep -v esp | sort | tail -1) os-image.qcow2
	incus image import --alias incus-os test/metadata.tar.xz os-image.qcow2
	rm os-image.qcow2

	incus init --vm incus-os test-incus-os \
		-c security.secureboot=false \
		-c limits.cpu=4 \
		-c limits.memory=8GiB \
		-d root,size=50GiB
	incus config device add test-incus-os vtpm tpm
	incus start test-incus-os --console
	sleep 3
	incus console test-incus-os

.PHONY: test-extensions
test-extensions:
	incus exec test-incus-os -- mkdir -p /var/lib/extensions
	incus exec test-incus-os -- systemd-sysext list
	incus file push mkosi.output/debug.raw test-incus-os/var/lib/extensions/
	incus file push mkosi.output/incus.raw test-incus-os/var/lib/extensions/
	incus exec test-incus-os -- systemd-sysext list
	incus exec test-incus-os -- systemd-sysext merge
	incus exec test-incus-os -- systemd-sysusers
	incus exec test-incus-os -- systemctl enable --now incus-lxcfs incus-startup incus-user incus-user.socket incus incus.socket
	incus exec test-incus-os -- incus admin init --auto

.PHONY: test-update
test-update:
	incus file create test-incus-os/var/lib/updates/ --type=directory
	incus file push $(shell ls mkosi.output/IncusOS_*.efi | sort | tail -1) test-incus-os/var/lib/updates/
	incus file push $(shell ls mkosi.output/IncusOS_*.usr-x86-64.* | sort | tail -1) test-incus-os/var/lib/updates/
	incus file push $(shell ls mkosi.output/IncusOS_*.usr-x86-64-verity.* | sort | tail -1) test-incus-os/var/lib/updates/
	incus file push $(shell ls mkosi.output/IncusOS_*.usr-x86-64-verity-sig.* | sort | tail -1) test-incus-os/var/lib/updates/
	incus file push mkosi.sysupdate/* test-incus-os/var/lib/updates/
	incus exec test-incus-os -- /usr/lib/systemd/systemd-sysupdate --definitions=/var/lib/updates/
	incus exec test-incus-os -- /usr/lib/systemd/systemd-sysupdate --definitions=/var/lib/updates/ update
	incus exec test-incus-os -- systemctl reboot
