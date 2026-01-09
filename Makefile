GO ?= go
SPHINXENV=doc/.sphinx/venv/bin/activate
SPHINXPIPPATH=doc/.sphinx/venv/bin/pip

.PHONY: default
default: build

.PHONY: clean
clean:
	sudo -E rm -Rf .cache/ certs/efi/updates/*.tar.gz mkosi.output/ mkosi.packages/*.deb
	sudo -E $(shell command -v mkosi) clean

.PHONY: incus-osd
incus-osd:
	(cd incus-osd && go build ./cmd/incus-osd)
	strip incus-osd/incus-osd

.PHONY: flasher-tool
flasher-tool:
	(cd incus-osd && go build ./cmd/flasher-tool)
	strip incus-osd/flasher-tool

.PHONY: generate-manifests
generate-manifests:
	(cd incus-osd && go build ./cmd/generate-manifests)
	strip incus-osd/generate-manifests

.PHONY: incusos-initrd-utils
incusos-initrd-utils:
	(cd incus-osd && go build ./cmd/incusos-initrd-utils)
	strip incus-osd/incusos-initrd-utils

.PHONY: initrd-deb-package
initrd-deb-package: inject-system-certs incusos-initrd-utils
	$(eval OSNAME := $(shell grep "ImageId=" mkosi.conf | cut -d '=' -f 2))
	cp incus-osd/incusos-initrd-utils mkosi.packages/incusos-initrd-utils/
	(cd mkosi.packages/incusos-initrd-utils && cp initrd-startup-checks.service.in initrd-startup-checks.service && sed -i -e "s/@OSNAME@/${OSNAME}/" initrd-startup-checks.service && debuild)
	rm -rf mkosi.packages/incusos-initrd-utils/debian/.debhelper/  mkosi.packages/incusos-initrd-utils/debian/debhelper-build-stamp \
          mkosi.packages/incusos-initrd-utils/debian/files \mkosi.packages/incusos-initrd-utils/debian/incusos-initrd-utils.postrm.debhelper \
          mkosi.packages/incusos-initrd-utils/debian/incusos-initrd-utils.substvars mkosi.packages/incusos-initrd-utils/debian/incusos-initrd-utils/ \
          mkosi.packages/incusos-initrd-utils_*.dsc mkosi.packages/incusos-initrd-utils_*.tar.xz mkosi.packages/incusos-initrd-utils_*.build \
          mkosi.packages/incusos-initrd-utils_*.buildinfo mkosi.packages/incusos-initrd-utils_*.changes

.PHONY: microcode-metapackage-deb-package
microcode-metapackage-deb-package:
	(cd mkosi.packages/microcode-metapackage && debuild)
	rm -rf mkosi.packages/microcode-metapackage/debian/.debhelper/  mkosi.packages/microcode-metapackage/debian/debhelper-build-stamp \
          mkosi.packages/microcode-metapackage/debian/files \mkosi.packages/microcode-metapackage/debian/microcode-metapackage.postrm.debhelper \
          mkosi.packages/microcode-metapackage/debian/microcode-metapackage.substvars mkosi.packages/microcode-metapackage/debian/microcode-metapackage/ \
          mkosi.packages/microcode-metapackage_*.dsc mkosi.packages/microcode-metapackage_*.tar.xz mkosi.packages/microcode-metapackage_*.build \
          mkosi.packages/microcode-metapackage_*.buildinfo mkosi.packages/microcode-metapackage_*.changes

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

.PHONY: inject-system-certs
inject-system-certs:
ifeq (, $(shell which mkosi))
	@echo "mkosi couldn't be found, please install it and try again"
	exit 1
endif

	-mkosi genkey
	mkdir -p mkosi.images/base/mkosi.extra/boot/EFI/
	openssl x509 -in mkosi.crt -out mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der -outform DER

	./scripts/inject-system-certs.sh

.PHONY: build
build: incus-osd flasher-tool generate-manifests initrd-deb-package microcode-metapackage-deb-package
	cd app-build/ && ./build-applications.py

	# Limit building of the Migration Manager worker image to amd64, since the vmware vddk isn't available for arm64.
ifeq ($(shell uname -m), x86_64)
	cd app-build/migration-manager/worker && make build

	mkdir -p mkosi.images/migration-manager/mkosi.extra/usr/share/migration-manager/images/
	cp app-build/migration-manager/worker/mkosi.output/migration-manager-worker.raw mkosi.images/migration-manager/mkosi.extra/usr/share/migration-manager/images/worker-x86_64.img
endif

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
	incus file push mkosi.output/gpu-support.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus-ceph.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus-linstor.raw test-incus-os/root/updates/
	incus file push mkosi.output/migration-manager.raw test-incus-os/root/updates/
	incus file push mkosi.output/operations-center.raw test-incus-os/root/updates/

	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system/update/:check -X POST

.PHONY: test-update
test-update:
	$(eval RELEASE := $(shell ls mkosi.output/*.efi | sed -e "s/.*_//g" -e "s/.efi//g" | sort -n | tail -1))
	incus exec test-incus-os -- mkdir -p /root/updates
	echo ${RELEASE} | incus file push - test-incus-os/root/updates/RELEASE

	incus file push mkosi.output/IncusOS_${RELEASE}.efi test-incus-os/root/updates/
	incus file push mkosi.output/IncusOS_${RELEASE}.usr* test-incus-os/root/updates/
	incus file push mkosi.output/debug.raw test-incus-os/root/updates/
	incus file push mkosi.output/gpu-support.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus-ceph.raw test-incus-os/root/updates/
	incus file push mkosi.output/incus-linstor.raw test-incus-os/root/updates/
	incus file push mkosi.output/migration-manager.raw test-incus-os/root/updates/
	incus file push mkosi.output/operations-center.raw test-incus-os/root/updates/

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

.PHONY: update-app-versions
update-app-versions:
	cd app-build && ./update-application-tags.py

.PHONY: update-api
update-api:
	$(GO) install -v -x github.com/go-swagger/go-swagger/cmd/swagger@master
	swagger generate spec -o doc/rest-api.yaml -w ./incus-osd/cmd/incus-osd -m

.PHONY: doc-setup
doc-setup:
	@echo "Setting up documentation build environment"
	python3 -m venv doc/.sphinx/venv
	. $(SPHINXENV) ; pip install --require-virtualenv --upgrade -r doc/.sphinx/requirements.txt --log doc/.sphinx/venv/pip_install.log
	@test ! -f doc/.sphinx/venv/pip_list.txt || \
        mv doc/.sphinx/venv/pip_list.txt doc/.sphinx/venv/pip_list.txt.bak
	$(SPHINXPIPPATH) list --local --format=freeze > doc/.sphinx/venv/pip_list.txt
	rm -Rf doc/html
	rm -Rf doc/.sphinx/.doctrees

.PHONY: doc
doc: doc-setup doc-incremental

.PHONY: doc-incremental
doc-incremental:
	@echo "Build the documentation"
	. $(SPHINXENV) ; sphinx-build -c doc/ -b dirhtml doc/ doc/html/ -d doc/.sphinx/.doctrees -w doc/.sphinx/warnings.txt
	cp doc/rest-api.yaml doc/html/

.PHONY: doc-serve
doc-serve:
	cd doc/html; python3 -m http.server 8001

.PHONY: doc-spellcheck
doc-spellcheck: doc
	. $(SPHINXENV) ; python3 -m pyspelling -c doc/.sphinx/spellingcheck.yaml

.PHONY: doc-linkcheck
doc-linkcheck: doc-setup
	. $(SPHINXENV) ; LOCAL_SPHINX_BUILD=True sphinx-build -c doc/ -b linkcheck doc/ doc/html/ -d doc/.sphinx/.doctrees

.PHONY: doc-lint
doc-lint:
	doc/.sphinx/.markdownlint/doc-lint.sh
