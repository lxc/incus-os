.PHONY: default
default: build

ifeq (, $(shell which mkosi))
$(error "mkosi couldn't be found, please install it and try again")
endif

.PHONE: clean
clean:
	sudo -E rm -Rf .cache/
	sudo -E $(shell command -v mkosi) clean

.PHONY: build
build:
	-mkosi genkey
	sudo rm -Rf mkosi.output
	sudo -E $(shell command -v mkosi) --cache-dir .cache/ build

.PHONY: test
test:
	incus delete -f test-incus-os || true
	incus image delete incus-os || true

	qemu-img convert -f raw -O qcow2 os-image.raw os-image.qcow2
	incus image import --alias incus-os test/metadata.tar.xz os-image.qcow2
	rm os-image.qcow2

	incus init --vm incus-os test-incus-os \
		-c security.secureboot=false \
		-c limits.cpu=4 \
		-c limits.memory=8GiB
	incus config device add test-incus-os vtpm tpm
	incus start test-incus-os --console
