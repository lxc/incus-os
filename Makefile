GO ?= go
SPHINXENV=.sphinx/venv/bin/activate

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
build: incus-osd flasher-tool initrd-deb-package
ifeq (, $(shell which mkosi))
	@echo "mkosi couldn't be found, please install it and try again"
	exit 1
endif

	-mkosi genkey
	mkdir -p mkosi.images/base/mkosi.extra/boot/EFI/
	openssl x509 -in mkosi.crt -out mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der -outform DER

	cd app-build/ && ./build-applications.sh

	mkdir -p mkosi.images/base/mkosi.extra/usr/local/bin/
	cp incus-osd/incus-osd mkosi.images/base/mkosi.extra/usr/local/bin/
	cp app-build/kpx/kpx mkosi.images/base/mkosi.extra/usr/local/bin/
	cp app-build/tailscale/tailscaled mkosi.images/base/mkosi.extra/usr/local/bin/
	rm -f mkosi.images/base/mkosi.extra/usr/local/bin/tailscale
	ln -s tailscaled mkosi.images/base/mkosi.extra/usr/local/bin/tailscale

	mkdir -p mkosi.images/migration-manager/mkosi.extra/usr/local/bin/
	mkdir -p mkosi.images/migration-manager/mkosi.extra/usr/lib/migration-manager/
	mkdir -p mkosi.images/migration-manager/mkosi.extra/usr/share/migration-manager/ui/
	cp app-build/migration-manager/migration-managerd mkosi.images/migration-manager/mkosi.extra/usr/local/bin/
	cp app-build/migration-manager/migration-manager mkosi.images/migration-manager/mkosi.extra/usr/local/bin/
	cp app-build/migration-manager/migration-manager-worker mkosi.images/migration-manager/mkosi.extra/usr/lib/migration-manager/

	cp -r app-build/migration-manager/ui/dist/* mkosi.images/migration-manager/mkosi.extra/usr/share/migration-manager/ui/

	# Allow copy of the migration manager worker image to fail, since it only exists for amd64 at the moment.
	mkdir -p mkosi.images/migration-manager/mkosi.extra/usr/share/migration-manager/images/
	-cp app-build/migration-manager/worker/mkosi.output/migration-manager-worker.raw mkosi.images/migration-manager/mkosi.extra/usr/share/migration-manager/images/worker-$$(uname -m).img

	mkdir -p mkosi.images/operations-center/mkosi.extra/usr/local/bin/
	mkdir -p mkosi.images/operations-center/mkosi.extra/usr/share/operations-center/ui/
	mkdir -p mkosi.images/operations-center/mkosi.extra/usr/share/terraform/plugins/registry.opentofu.org/
	cp app-build/opentofu/tofu mkosi.images/operations-center/mkosi.extra/usr/local/bin/
	cp app-build/operations-center/operations-centerd mkosi.images/operations-center/mkosi.extra/usr/local/bin/
	cp app-build/operations-center/operations-center mkosi.images/operations-center/mkosi.extra/usr/local/bin/
	cp -r app-build/operations-center/ui/dist/* mkosi.images/operations-center/mkosi.extra/usr/share/operations-center/ui/
	cp -r app-build/terraform-provider-null/hashicorp/ mkosi.images/operations-center/mkosi.extra/usr/share/terraform/plugins/registry.opentofu.org/
	cp -r app-build/terraform-provider-random/hashicorp/ mkosi.images/operations-center/mkosi.extra/usr/share/terraform/plugins/registry.opentofu.org/
	cp -r app-build/terraform-provider-time/hashicorp/ mkosi.images/operations-center/mkosi.extra/usr/share/terraform/plugins/registry.opentofu.org/
	cp -r app-build/terraform-provider-incus/lxc/ mkosi.images/operations-center/mkosi.extra/usr/share/terraform/plugins/registry.opentofu.org/

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

	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system/update/:check -X POST

.PHONY: test-update
test-update:
	$(eval RELEASE := $(shell ls mkosi.output/*.efi | sed -e "s/.*_//g" -e "s/.efi//g" | sort -n | tail -1))
	incus exec test-incus-os -- mkdir -p /root/updates
	echo ${RELEASE} | incus file push - test-incus-os/root/updates/RELEASE

	incus file push mkosi.output/IncusOS_${RELEASE}.efi test-incus-os/root/updates/
	incus file push mkosi.output/IncusOS_${RELEASE}.usr* test-incus-os/root/updates/
	incus file push mkosi.output/debug.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus.raw test-incus-os/root/updates/

	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system/update/:check -X POST

.PHONY: test-update-sb-keys
test-update-sb-keys:
	$(eval RELEASE := $(shell ls mkosi.output/*.efi | sed -e "s/.*_//g" -e "s/.efi//g" | sort -n | tail -1))
	incus exec test-incus-os -- mkdir -p /root/updates
	echo ${RELEASE} | incus file push - test-incus-os/root/updates/RELEASE

	cd certs/efi/updates/ && tar cf SecureBootKeys_${RELEASE}.tar *.auth
	incus file push certs/efi/updates/*.tar test-incus-os/root/updates/

	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system/update/:check -X POST

.PHONY: update-gomod
update-gomod:
	cd incus-osd && go get -t -v -u ./...
	cd incus-osd && go mod tidy --go=1.24.7
	cd incus-osd && go get toolchain@none

.PHONY: doc-setup
doc-setup:
	@echo "Setting up documentation build environment"
	python3 -m venv .sphinx/venv
	. $(SPHINXENV) ; pip install --upgrade -r .sphinx/requirements.txt
	mkdir -p .sphinx/deps/ .sphinx/themes/
	wget -N -P .sphinx/_static/download https://linuxcontainers.org/static/img/favicon.ico https://linuxcontainers.org/static/img/containers.png https://linuxcontainers.org/static/img/containers.small.png
	rm -Rf doc/html

.PHONY: doc
doc: doc-setup doc-incremental

.PHONY: doc-incremental
doc-incremental:
	@echo "Build the documentation"
	. $(SPHINXENV) ; sphinx-build -c .sphinx/ -b dirhtml doc/ doc/html/ -w .sphinx/warnings.txt

.PHONY: doc-serve
doc-serve:
	cd doc/html; python3 -m http.server 8001

.PHONY: doc-spellcheck
doc-spellcheck: doc
	. $(SPHINXENV) ; python3 -m pyspelling -c .sphinx/.spellcheck.yaml

.PHONY: doc-linkcheck
doc-linkcheck: doc-setup
	. $(SPHINXENV) ; sphinx-build -c .sphinx/ -b linkcheck doc/ doc/html/

.PHONY: doc-lint
doc-lint:
	.sphinx/.markdownlint/doc-lint.sh
