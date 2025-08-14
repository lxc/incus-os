GO ?= go

.PHONY: default
default: build

.PHONY: clean
clean:
	sudo -E rm -Rf .cache/ certs/efi/updates/*.tar.gz mkosi.output/ mkosi.packages/initrd-tmpfs-root_*_all.deb
	sudo -E $(shell command -v mkosi) clean

.PHONY: incus-osd
incus-osd:
	(cd incus-osd && go build ./cmd/incus-osd)
	strip incus-osd/incus-osd

.PHONY: flasher-tool
flasher-tool:
	(cd incus-osd && go build ./cmd/flasher-tool)
	strip incus-osd/flasher-tool

.PHONY: kpx
kpx:
	$(eval KPX_VERSION := 1.11.0)
ifeq (,$(wildcard incus-osd/kpx/))
	git clone https://github.com/momiji/kpx incus-osd/kpx/ --depth 1 -b "v${KPX_VERSION}"
else
	(cd incus-osd/kpx && git reset --hard && git fetch --depth 1 origin "v${KPX_VERSION}":refs/tags/"v${KPX_VERSION}" && git checkout "v${KPX_VERSION}")
endif

	(cd incus-osd/kpx && patch -p1 < ../../patches/kpx-0001-Enable-IPv6-support.patch)

	(cd incus-osd/kpx/cli && go build -o kpx -ldflags="-s -w -X github.com/momiji/kpx.AppVersion=${KPX_VERSION}")
	strip incus-osd/kpx/cli/kpx

.PHONY: initrd-deb-package
initrd-deb-package:
	$(eval OSNAME := $(shell grep "ImageId=" mkosi.conf | cut -d '=' -f 2))
	(cd mkosi.packages/initrd-tmpfs-root && cp initrd-message.service.in initrd-message.service && sed -i -e "s/@OSNAME@/${OSNAME}/" initrd-message.service && debuild)
	rm -rf mkosi.packages/initrd-tmpfs-root/debian/.debhelper/  mkosi.packages/initrd-tmpfs-root/debian/debhelper-build-stamp \
          mkosi.packages/initrd-tmpfs-root/debian/files \mkosi.packages/initrd-tmpfs-root/debian/initrd-tmpfs-root.postrm.debhelper \
          mkosi.packages/initrd-tmpfs-root/debian/initrd-tmpfs-root.substvars mkosi.packages/initrd-tmpfs-root/debian/initrd-tmpfs-root/ \
          mkosi.packages/initrd-tmpfs-root_*.dsc mkosi.packages/initrd-tmpfs-root_*.tar.xz mkosi.packages/initrd-tmpfs-root_*.build \
          mkosi.packages/initrd-tmpfs-root_*.buildinfo mkosi.packages/initrd-tmpfs-root_*.changes

.PHONY: static-analysis
static-analysis:
ifeq ($(shell command -v go-licenses),)
	(cd / ; $(GO) install -v -x github.com/google/go-licenses@latest)
endif
ifeq ($(shell command -v golangci-lint),)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin
endif

	cd incus-osd/ && run-parts $(shell run-parts -V >/dev/null 2>&1 && echo -n "--verbose --exit-on-error --regex '.sh'") ../scripts/lint

.PHONY: generate-test-certs
generate-test-certs:
ifeq (,$(wildcard ./certs/))
	./scripts/test/generate-test-certificates.sh
	./scripts/test/generate-secure-boot-vars.sh
	./scripts/test/switch-secure-boot-signing-key.sh 1
endif

.PHONY: build
build: incus-osd flasher-tool kpx initrd-deb-package
ifeq (, $(shell which mkosi))
	@echo "mkosi couldn't be found, please install it and try again"
	exit 1
endif

	-mkosi genkey
	mkdir -p mkosi.images/base/mkosi.extra/boot/EFI/
	openssl x509 -in mkosi.crt -out mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der -outform DER
	mkdir -p mkosi.images/base/mkosi.extra/usr/local/bin/
	cp incus-osd/incus-osd mkosi.images/base/mkosi.extra/usr/local/bin/
	cp incus-osd/kpx/cli/kpx mkosi.images/base/mkosi.extra/usr/local/bin/
	sudo rm -Rf mkosi.output/base* mkosi.output/debug* mkosi.output/incus*
	sudo -E $(shell command -v mkosi) --cache-dir .cache/ build
	sudo chown $(shell id -u):$(shell id -g) mkosi.output

ifneq (,$(wildcard ./certs/))
	# For some reason getting the image name via $(shell ...) is always empty here?
	sudo ./scripts/inject-secure-boot-vars.sh `ls mkosi.output/IncusOS_*.raw | grep -v usr | grep -v esp | sort | tail -1`
endif

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
	incus config device add test-incus-os boot-media disk source=$$(pwd)/mkosi.output/IncusOS_boot_media.img io.bus=usb boot.priority=10 readonly=false

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

	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system -X PUT -d '{"action": "update"}'

.PHONY: test-update
test-update:
	$(eval RELEASE := $(shell ls mkosi.output/*.efi | sed -e "s/.*_//g" -e "s/.efi//g" | sort -n | tail -1))
	incus exec test-incus-os -- mkdir -p /root/updates
	echo ${RELEASE} | incus file push - test-incus-os/root/updates/RELEASE

	incus file push mkosi.output/IncusOS_${RELEASE}.efi test-incus-os/root/updates/
	incus file push mkosi.output/IncusOS_${RELEASE}.usr* test-incus-os/root/updates/
	incus file push mkosi.output/debug.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus.raw test-incus-os/root/updates/

	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system -X PUT -d '{"action": "update"}'

.PHONY: test-update-sb-keys
test-update-sb-keys:
	$(eval RELEASE := $(shell ls mkosi.output/*.efi | sed -e "s/.*_//g" -e "s/.efi//g" | sort -n | tail -1))
	incus exec test-incus-os -- mkdir -p /root/updates
	echo ${RELEASE} | incus file push - test-incus-os/root/updates/RELEASE

	cd certs/efi/updates/ && tar czf IncusOS_SecureBootKeys_${RELEASE}.tar.gz *.auth
	incus file push certs/efi/updates/*.tar.gz test-incus-os/root/updates/

	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system -X PUT -d '{"action": "update"}'

.PHONY: update-gomod
update-gomod:
	cd incus-osd && go get -t -v -u ./...
	cd incus-osd && go get github.com/go-jose/go-jose/v4@v4.0.5
	cd incus-osd && go mod tidy --go=1.23.7
	cd incus-osd && go get toolchain@none
