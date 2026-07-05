GO ?= go
SPHINXENV=doc/.sphinx/venv/bin/activate
SPHINXPIPPATH=doc/.sphinx/venv/bin/pip

ARCH=$(shell if [ "$$(uname -m)" = "x86_64" ]; then echo "amd64"; else echo "arm64"; fi)
OSNAME=$(shell grep "ImageId=" mkosi.conf | cut -d '=' -f 2)
RELEASE=$(shell ls mkosi.output/*.efi 2>/dev/null | sed -e "s/.*_//g" -e "s/.efi//g" | sort -n | tail -1)

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

.PHONY: image-publisher
image-publisher:
	(cd incus-osd && go build ./cmd/image-publisher)
	strip incus-osd/image-publisher

.PHONY: initrd-utils
initrd-utils:
	(cd incus-osd && go build ./cmd/initrd-utils)
	strip incus-osd/initrd-utils

.PHONY: initrd-deb-package
initrd-deb-package: initrd-utils
	cp incus-osd/initrd-utils mkosi.packages/initrd-utils/
	rm -rf mkosi.packages/initrd-utils/repart.d/ && cp -r mkosi.images/base/mkosi.extra/usr/lib/repart.d/ mkosi.packages/initrd-utils/
	(cd mkosi.packages/initrd-utils && cp os-release.in os-release && sed -i -e "s/@IMAGE_ID@/${OSNAME}/" os-release)
	(cd mkosi.packages/initrd-utils && debuild)
	rm -rf mkosi.packages/initrd-utils/debian/.debhelper/  mkosi.packages/initrd-utils/debian/debhelper-build-stamp \
          mkosi.packages/initrd-utils/debian/files \mkosi.packages/initrd-utils/debian/initrd-utils.postrm.debhelper \
          mkosi.packages/initrd-utils/debian/initrd-utils.substvars mkosi.packages/initrd-utils/debian/initrd-utils/ \
          mkosi.packages/initrd-utils_*.dsc mkosi.packages/initrd-utils_*.tar.xz mkosi.packages/initrd-utils_*.build \
          mkosi.packages/initrd-utils_*.buildinfo mkosi.packages/initrd-utils_*.changes

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
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin
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

# If no mkosi.crt file exists, auto-generate a set of test certificates for development use.
# The CI system will pre-populate mkosi.crt and mkosi.key when building a production release.
ifeq (,$(wildcard ./mkosi.crt))
	make generate-test-certs
endif

	mkdir -p mkosi.images/base/mkosi.extra/boot/EFI/
	openssl x509 -in mkosi.crt -out mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der -outform DER

	rm -rf mkosi.images/base/mkosi.extra/usr/lib/verity.d/
	mkdir -p mkosi.images/base/mkosi.extra/usr/lib/verity.d/
	cp incus-osd/certs/files/secureboot-DB-*.crt mkosi.images/base/mkosi.extra/usr/lib/verity.d/

.PHONY: build
build: inject-system-certs incus-osd flasher-tool generate-manifests image-publisher initrd-deb-package microcode-metapackage-deb-package
	cd app-build/ && ./build-applications.py

	sudo rm -Rf mkosi.output/base* mkosi.output/debug* mkosi.output/incus*
	sudo -E $(shell command -v mkosi) --cache-dir .cache/ build
	sudo chown $(shell id -u):$(shell id -g) mkosi.output

	# For some reason getting the image name via $(shell ...) is always empty here?
	sudo ./scripts/inject-secure-boot-vars.sh `ls mkosi.output/${OSNAME}_*.raw | grep -v usr | grep -v esp | sort | tail -1`

.PHONY: build-iso
build-iso: build
	sudo ./scripts/convert-img-to-iso.sh $(shell ls mkosi.output/${OSNAME}_*.raw | grep -v usr | grep -v esp | sort | tail -1)

.PHONY: test
test: publish-local-update start-local-image-server
	# Cleanup
	incus delete -f test-incus-os || true
	rm -f mkosi.output/${OSNAME}_boot_media.img

	# Inject a custom provider seed to point the VM to our local images server
	$(eval SERVER_IP := $(shell incus network list incusbr0 -c 4 -f csv | cut -d '/' -f 1))
	cp test/seed.install.tar test/seed-test.install.tar
	echo '{"name":"images","config":{"server_url":"http://${SERVER_IP}:8123/os"}}' > provider.json
	echo '{"channel":"testing"}' > update.json
	tar -rf test/seed-test.install.tar provider.json update.json
	rm provider.json update.json

	# Prepare the install media
	cp $(shell ls mkosi.output/${OSNAME}_*.raw | grep -v usr | grep -v esp | sort | tail -1) mkosi.output/${OSNAME}_boot_media.img
	dd if=test/seed-test.install.tar of=mkosi.output/${OSNAME}_boot_media.img seek=4196352 bs=512 conv=notrunc
	rm test/seed-test.install.tar

	# Create the VM
	incus init --empty --vm test-incus-os \
		-c security.secureboot=false \
		-c limits.cpu=4 \
		-c limits.memory=8GiB \
		-d root,size=50GiB
	incus config device add test-incus-os vtpm tpm
	incus config device add test-incus-os boot-media disk source=$$(pwd)/mkosi.output/${OSNAME}_boot_media.img io.bus=usb boot.priority=10 readonly=false

	# Enable full incus-agent capability
	incus config set test-incus-os systemd.credential.fully-enable-incus-agent=true

	# Wait for installation to complete
	incus start test-incus-os --console
	@sleep 5 # Wait for VM self-reboot
	incus config set test-incus-os volatile.vm.needs_reset=true # Ensure the VM performs a full reset after the install completes
	incus console test-incus-os
	@sleep 5 # Wait for VM self-reboot
	@clear # Clear the console

	# Remove install media
	incus stop -f test-incus-os
	incus config device remove test-incus-os boot-media

	# Start the installed system
	incus start test-incus-os --console

.PHONY: test-iso
test-iso: publish-local-update start-local-image-server
	# Cleanup
	incus delete -f test-incus-os || true
	incus storage volume delete default ${OSNAME}_boot_media.iso || true
	rm -f mkosi.output/${OSNAME}_boot_media.iso

	# Inject a custom provider seed to point the VM to our local images server
	$(eval SERVER_IP := $(shell incus network list incusbr0 -c 4 -f csv | cut -d '/' -f 1))
	cp test/seed.install.tar test/seed-test.install.tar
	echo '{"name":"images","config":{"server_url":"http://${SERVER_IP}:8123/os"}}' > provider.json
	echo '{"channel":"testing"}' > update.json
	tar -rf test/seed-test.install.tar provider.json update.json
	rm provider.json update.json

	# Prepare the install media
	cp $(shell ls mkosi.output/${OSNAME}_*.iso | grep -v usr | grep -v esp | sort | tail -1) mkosi.output/${OSNAME}_boot_media.iso
	dd if=test/seed-test.install.tar of=mkosi.output/${OSNAME}_boot_media.iso seek=4196352 bs=512 conv=notrunc
	incus storage volume import default mkosi.output/${OSNAME}_boot_media.iso ${OSNAME}_boot_media.iso --type=iso
	rm test/seed-test.install.tar

	# Create the VM
	incus init --empty --vm test-incus-os \
		-c security.secureboot=false \
		-c limits.cpu=4 \
		-c limits.memory=8GiB \
		-d root,size=50GiB
	incus config device add test-incus-os vtpm tpm
	incus config device add test-incus-os boot-media disk pool=default source=${OSNAME}_boot_media.iso boot.priority=10

	# Enable full incus-agent capability
	incus config set test-incus-os systemd.credential.fully-enable-incus-agent=true

	# Wait for installation to complete
	incus start test-incus-os --console
	@sleep 5 # Wait for VM self-reboot
	incus config set test-incus-os volatile.vm.needs_reset=true # Ensure the VM performs a full reset after the install completes
	incus console test-incus-os
	@sleep 5 # Wait for VM self-reboot
	@clear # Clear the console

	# Remove install media
	incus stop -f test-incus-os
	incus config device remove test-incus-os boot-media

	# Start the installed system
	incus start test-incus-os --console

.PHONY: test-update
test-update: publish-local-update start-local-image-server
	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system/update/:check -X POST

.PHONY: publish-local-update
publish-local-update:
# Don't perform publish steps if already available locally
ifeq (,$(wildcard ./local-image-server/${RELEASE}/))
	rm -rf ./upload/ ${OSNAME}-*-${ARCH}.zip
	./scripts/prepare-upload.sh ${RELEASE}

	(cd upload && for i in *; do gzip -1 "$${i}"; done)

	zip -j ${OSNAME}-${RELEASE}-${ARCH}.zip upload/*

	SIG_KEY=./certs/cas/root-E1.key SIG_CERTIFICATE=./incus-osd/certs/files/root-E1.crt SIG_CHAIN=./mkosi.crt ./incus-osd/image-publisher sync ./local-image-server/ ./${OSNAME}-${RELEASE}-${ARCH}.zip

	rm -rf ./upload/ ${OSNAME}-*-${ARCH}.zip
endif

.PHONY: start-local-image-server
start-local-image-server:
	$(eval SERVER_IP := $(shell incus network list incusbr0 -c 4 -f csv | cut -d '/' -f 1))

	# Cleanup stale pid
	if [ -f ./scripts/test/local-images-server.pid ] && [ ! -d /proc/$$(cat ./scripts/test/local-images-server.pid) ]; then rm ./scripts/test/local-images-server.pid; fi

	./scripts/test/local-images-server.py ${SERVER_IP} &

.PHONY: test-update-sb-keys
test-update-sb-keys:
	rm -rf ./upload/ ${OSNAME}-*-${ARCH}.zip ./sb.tar.gz ./local-image-server/${RELEASE}/
	./scripts/prepare-upload.sh ${RELEASE}

	(cd upload && for i in *; do gzip -1 "$${i}"; done)

	zip -j ${OSNAME}-${RELEASE}-${ARCH}.zip upload/*
	(cd certs/efi/updates/ && tar czf ../../../sb.tar.gz *.auth)

	SIG_KEY=./certs/cas/root-E1.key SIG_CERTIFICATE=./incus-osd/certs/files/root-E1.crt SIG_CHAIN=./mkosi.crt UPDATE_SECUREBOOT=./sb.tar.gz ./incus-osd/image-publisher sync ./local-image-server/ ./${OSNAME}-${RELEASE}-${ARCH}.zip

	rm -rf ./upload/ ${OSNAME}-*-${ARCH}.zip ./sb.tar.gz

	incus exec test-incus-os -- curl --unix-socket /run/incus-os/unix.socket http://localhost/1.0/system/update/:check -X POST

.PHONY: update-gomod
update-gomod:
	cd incus-osd && go get -t -v -u ./...
	cd incus-osd && go get tailscale.com@v1.94.2
	sed -i "/go-json-experiment/d" incus-osd/go.mod
	cd incus-osd && go mod tidy --go=1.25.11
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

	$(eval OSNAME_LOWER := $(shell echo ${OSNAME} | tr '[:upper:]' '[:lower:]'))
	$(eval OSNAME_UPPER := $(shell echo ${OSNAME} | tr '[:lower:]' '[:upper:]'))

	find ./doc/ -name '*.md' | xargs sed -i -e "s/IncusOS/${OSNAME}/g"
	find ./doc/ -name '*.md' | xargs sed -i -e "s/INCUSOS/${OSNAME_UPPER}/g"
	find ./doc/ -name '*.md' | xargs sed -i -e "s/incusos/${OSNAME_LOWER}/g"

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
