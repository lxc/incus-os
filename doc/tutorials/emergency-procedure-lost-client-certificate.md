# Emergency Procedure for a Lost Client Certificate

Losing the trusted client certificate/key pair will result in loss of access to the IncusOS API. By default, Incus stores its client certificate in `~/.config/incus/`, and the contents should be backed up appropriately.

It is possible to manually enroll a new trusted client certificate by hand, which will restore access to your IncusOS server.

## Requirements

* You must have an encryption recovery key for the IncusOS server, either the one automatically generated on first boot or a custom one set via the API. Without this recovery key, it will be impossible to decrypt the main IncusOS partition and you will have no choice but to reinstall IncusOS.

* You must have physical (console) access to the IncusOS server.

* The following steps assume Incus was installed on IncusOS and is the application responsible for handling client authentication. Recovery steps for Migration Manager and Operations Center will be different and are not covered in this document.

## Steps

1. Generate a new client certificate/key pair. You can do this easily via `incus remote get-client-certificate`. Make sure to backup your new client credentials. 🙂 Copy the certificate to a USB stick or other removable media for use in a later step.

1. Disable Secure Boot on the IncusOS server. Don't clear any certificates, just set the Secure Boot mode to disabled. The exact steps to accomplish this vary from manufacturer to manufacturer.

1. Boot a live Linux image of your choice on the IncusOS server. Any modern image will work, as long as the `cryptsetup` command is available.

1. Decrypt and mount the IncusOS root partition. We'll assume the main system drive is `/dev/sda`; adjust as appropriate for your system.

    ```
    cryptsetup luksOpen /dev/sda10 sda10_crypt
    mkdir -p /mnt/incusos/
    mount /dev/mapper/sda10_crypt /mnt/incusos/
    ```

1. Prepare an Incus database hotfix patch to enroll the new client certificate.

   ```
   CERT="/path/to/new/client.crt"
   CERT_CONTENTS=$(cat $CERT)
   FINGERPRINT=$(openssl x509 -in $CERT -noout -fingerprint -sha256 | tr -d ":" | sed "s/sha256 Fingerprint=//")

   echo "INSERT INTO certificates (fingerprint, type, name, certificate) VALUES (\"$FINGERPRINT\", 1, \"new-client-cert\", \"$CERT_CONTENTS\");" > /mnt/incusos/var/lib/incus/database/patch.global.sql
   ```

1. Unmount the IncusOS root partition.

   ```
   umount /mnt/incusos/
   cryptsetup luksClose /dev/mapper/sda10_crypt
   ```

1. Re-enable Secure Boot on the IncusOS server.

1. Upon reboot, IncusOS will boot normally and you will be able to authenticate with the new client certificate.
