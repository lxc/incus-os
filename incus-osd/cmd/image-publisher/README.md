## Publishing a local build

Here are the steps that can be taken to publish a local build of IncusOS. The resulting JSON metadata will be
signed with the local development certificates, allowing for updates to be published for local debugging and
development work.

### Generate manifests for the build

```
mkdir upload/
./incus-osd/generate-manifests .
mv upload/*.json mkosi.output/
```

### Compress build artifacts and create the zip archive

```
pushd ./mkosi.output
export RELEASE=$(ls *.efi | tr -dc '0-9')
find -maxdepth 1 -type f | xargs gzip
zip ../image-${RELEASE}-amd64.zip *.gz
popd
```

### Publish the local build, signing the JSON metadata with local development keys

```
SIG_KEY=./certs/cas/root-E1.key SIG_CERTIFICATE=./incus-osd/certs/files/root-E1.crt SIG_CHAIN=./incus-osd/certs/files/secureboot-DB-1-R1.crt ./incus-osd/image-publisher sync ./mirror/ ./image-${RELEASE}-amd64.zip
```

### Optionally, prepare a rescue-mode virtual disk

```
cp -r ./mirror/${RELEASE} ./update/

truncate --size 4GiB update.img
/sbin/sgdisk -n 1 -c 1:RESCUE_DATA ./update.img
/sbin/mkfs.vfat -S 512 --offset=2048 ./update.img
mcopy -s -i ./update.img@@1048576 update ::

incus config device add <vm> update disk source=$(pwd)/update.img
```
