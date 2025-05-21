.PHONY: default
default: build

ifeq (, $(shell which mkosi))
$(error "mkosi couldn't be found, please install it and try again")
endif

.PHONY: clean
clean:
	sudo -E rm -Rf .cache/ mkosi.output/ mkosi.packages/initrd-tmpfs-root_*_all.deb
	sudo -E $(shell command -v mkosi) clean

.PHONY: incus-osd
incus-osd:
	(cd incus-osd && go build ./cmd/incus-osd)
	strip incus-osd/incus-osd

.PHONY: initrd-deb-package
initrd-deb-package:
	(cd mkosi.packages/initrd-tmpfs-root && debuild)
	rm -rf mkosi.packages/initrd-tmpfs-root/debian/.debhelper/  mkosi.packages/initrd-tmpfs-root/debian/debhelper-build-stamp \
          mkosi.packages/initrd-tmpfs-root/debian/files \mkosi.packages/initrd-tmpfs-root/debian/initrd-tmpfs-root.postrm.debhelper \
          mkosi.packages/initrd-tmpfs-root/debian/initrd-tmpfs-root.substvars mkosi.packages/initrd-tmpfs-root/debian/initrd-tmpfs-root/ \
          mkosi.packages/initrd-tmpfs-root_*.dsc mkosi.packages/initrd-tmpfs-root_*.tar.xz mkosi.packages/initrd-tmpfs-root_*.build \
          mkosi.packages/initrd-tmpfs-root_*.buildinfo mkosi.packages/initrd-tmpfs-root_*.changes

.PHONY: static-analysis
static-analysis:
	(cd incus-osd && golangci-lint run)

.PHONY: build
build: incus-osd initrd-deb-package
	-mkosi genkey
	mkdir -p mkosi.images/base/mkosi.extra/boot/EFI/
	openssl x509 -in mkosi.crt -out mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der -outform DER
	mkdir -p mkosi.images/base/mkosi.extra/usr/local/bin/
	cp incus-osd/incus-osd mkosi.images/base/mkosi.extra/usr/local/bin/
	sudo rm -Rf mkosi.output/base* mkosi.output/debug* mkosi.output/incus*
	sudo -E $(shell command -v mkosi) --cache-dir .cache/ build
	sudo chown $(shell id -u):$(shell id -g) mkosi.output

.PHONY: build-iso
build-iso: build
	sudo ./scripts/convert-img-to-iso.sh $(shell ls mkosi.output/IncusOS_*.raw | grep -v usr | grep -v esp | sort | tail -1)

.PHONY: test
test:
	# Cleanup
	incus delete -f test-incus-os || true
	rm -f mkosi.output/IncusOS_boot_media.img

	# Prepare the install media
	cp $(shell ls mkosi.output/IncusOS_*.raw | grep -v usr | grep -v esp | sort | tail -1) mkosi.output/IncusOS_boot_media.img
	dd if=test/seed.install.tar of=mkosi.output/IncusOS_boot_media.img seek=4196352 bs=512 conv=notrunc

	# Create the VM
	incus init --empty --vm test-incus-os \
		-c security.secureboot=false \
		-c limits.cpu=4 \
		-c limits.memory=8GiB \
		-d root,size=50GiB
	incus config device add test-incus-os vtpm tpm
	incus config device add test-incus-os boot-media disk source=$$(pwd)/mkosi.output/IncusOS_boot_media.img boot.priority=10 readonly=false

	# Wait for installation to complete
	incus start test-incus-os --console
	@sleep 5 # Wait for VM self-reboot
	incus console test-incus-os
	@sleep 5 # Wait for VM self-reboot
	@clear # Clear the console

	# Remove install media
	incus stop -f test-incus-os
	incus config device remove test-incus-os boot-media

	# Start the installed system
	incus start test-incus-os --console

.PHONY: test-iso
test-iso:
	# Cleanup
	incus delete -f test-incus-os || true
	incus storage volume delete default IncusOS_boot_media.iso || true
	rm -f mkosi.output/IncusOS_boot_media.iso

	# Prepare the install media
	cp $(shell ls mkosi.output/IncusOS_*.iso | grep -v usr | grep -v esp | sort | tail -1) mkosi.output/IncusOS_boot_media.iso
	dd if=test/seed.install.tar of=mkosi.output/IncusOS_boot_media.iso seek=4196352 bs=512 conv=notrunc
	incus storage volume import default mkosi.output/IncusOS_boot_media.iso IncusOS_boot_media.iso --type=iso

	# Create the VM
	incus init --empty --vm test-incus-os \
		-c security.secureboot=false \
		-c limits.cpu=4 \
		-c limits.memory=8GiB \
		-d root,size=50GiB
	incus config device add test-incus-os vtpm tpm
	incus config device add test-incus-os boot-media disk pool=default source=IncusOS_boot_media.iso boot.priority=10

	# Wait for installation to complete
	incus start test-incus-os --console
	@sleep 5 # Wait for VM self-reboot
	incus console test-incus-os
	@sleep 5 # Wait for VM self-reboot
	@clear # Clear the console

	# Remove install media
	incus stop -f test-incus-os
	incus config device remove test-incus-os boot-media

	# Start the installed system
	incus start test-incus-os --console

.PHONY: test-applications
test-applications:
	$(eval RELEASE := $(shell ls mkosi.output/*.efi | sed -e "s/.*_//g" -e "s/.efi//g" | sort -n | tail -1))
	incus exec test-incus-os -- mkdir -p /root/updates
	echo ${RELEASE} | incus file push - test-incus-os/root/updates/RELEASE

	incus file push mkosi.output/debug.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus.raw test-incus-os/root/updates/

	incus exec test-incus-os -- systemctl restart incus-osd

.PHONY: test-update
test-update:
	$(eval RELEASE := $(shell ls mkosi.output/*.efi | sed -e "s/.*_//g" -e "s/.efi//g" | sort -n | tail -1))
	incus exec test-incus-os -- mkdir -p /root/updates
	echo ${RELEASE} | incus file push - test-incus-os/root/updates/RELEASE

	incus file push mkosi.output/IncusOS_${RELEASE}.efi test-incus-os/root/updates/
	incus file push mkosi.output/IncusOS_${RELEASE}.usr* test-incus-os/root/updates/
	incus file push mkosi.output/debug.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus.raw test-incus-os/root/updates/

	incus exec test-incus-os -- systemctl restart incus-osd

.PHONY: update-gomod
update-gomod:
	cd incus-osd && go get -t -v -u ./...
	cd incus-osd && go get github.com/go-jose/go-jose/v4@v4.0.5
	cd incus-osd && go mod tidy --go=1.23.7
	cd incus-osd && go get toolchain@none
