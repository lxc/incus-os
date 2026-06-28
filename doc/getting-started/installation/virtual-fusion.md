# Installing in a VMware Fusion virtual machine

IncusOS can be installed in a VMware Fusion virtual machine on MacOS.

```{note}
IncusOS requires the use of a TPM device and enabling Secure Boot.

In addition, the virtual machine needs to configured with custom Secure Boot keys before the initial start.
Otherwise, clearing NVRAM or a recreation would be required.
```

## Get and import install media

Follow the instructions to [get an IncusOS image](../download.md). This document will assume an ISO image is used.

## Configure networking

Because IncusOS needs runs nested containers and virtual machines, the
VMware network security policy must be pretty relaxed to allow for the
virtual machine to run its internal bridge.

Run the following command to enable Promiscuous Mode.

```
sudo touch "/Library/Preferences/VMware Fusion/promiscAuthorized"
```

## Create a new virtual machine

Select an ISO file or drag the ISO file into the VMware Fusion dialog.

![Select ISO file](../../images/fusion-begin.png)

Then select `Linux` and `Debian 13.x 64-bit Arm` as the operating system.

![Select operating system](../../images/fusion-distro.png)

You will be shown the virtual machine details, click on `Customize Settings` to
customize the virtual machine.

```{note}
Do not click on `Finish` yet, the virtual machine would be started and
would be in a bad Secure Boot state.
```

![Initial virtual machine details](../../images/fusion-initialcreation.png)

Set the name of the virtual machine and save.

![Virtual machine name](../../images/fusion-name.png)

You will be presented with the virtual machine settings dialog.

![Virtual machine settings](../../images/fusion-settings.png)

Customize the virtual machine hardware and set at least 4 CPUs and 4GiB of RAM

![Customize the resources](../../images/fusion-resources.png)

Customize the disk size and set it to at least 50GiB size.

![Customize the disk](../../images/fusion-disk.png)

Select encryption and choose the option to encrypt only the required files.

```{note}
You can also choose the option to encrypt all files. However, encrypting only the required files suffices.
```

![Encrypt the virtual machine](../../images/fusion-encryption.png)

You will be prompted to set a password, proceed to set it.

```{note}
For convenience, to avoid future password prompts when starting the virtual machine,
you can check `Remember Password`.
```

![Encryption password](../../images/fusion-password.png)

After encryption is enabled, click on `Add Device` on the Settings dialog and add a new TPM device.

![Add TPM device](../../images/fusion-tpm.png)

Select `Advanced` settings in the `Other` section of the Settings dialog and check the
`Enable UEFI Secure Boot` checkbox.

![Enable Secure Boot](../../images/fusion-secureboot.png)

Then close the settings dialog, the virtual machine creation is complete.

```{note}
Do not start the virtual machine at this point or it will create a bad Secure Boot state.
```

## Download the Secure Boot keys

Go to [`https://images.linuxcontainers.org/os/keys/`](https://images.linuxcontainers.org/os/keys/) and download:

- `secureboot-KEK-R1.der`
- `secureboot-DB-2025-R1.der`
- `secureboot-DB-2026-R1.der`

Then open Virtual Machine Library, right-click on the virtual machine and click `Show in Finder`.

![Show in Finder](../../images/fusion-revealfinder.png)

Then select the virtual machine's `.vmwarevm` file, right-click it and choose `Show Package Contents`.

![Show in Finder](../../images/fusion-packagecontents.png)

Copy the three secure boot key files into the directory.

![Secure Boot keys copied](../../images/fusion-securebootcopied.png)

Open Virtual Machine Library, right-click on the virtual machine and hold down the option key,
then click `Open Config File in Editor`.

![Show in Finder](../../images/fusion-configfile.png)

Paste these lines into the configuration file.

```
uefi.secureBoot.kekDefault.file0 = "secureboot-KEK-R1.der"
uefi.secureBoot.dbDefault.file0 = "secureboot-2025-R1.der"
uefi.secureBoot.dbDefault.file1 = "secureboot-2026-R1.der"
```

![Show in Finder](../../images/fusion-configfiledone.png)

## IncusOS installation

Start the virtual machine, and IncusOS will begin its installation.

![Installation boot](../../images/fusion-boot.png)

```{note}
VMware Fusion takes some time to hash the kernel image during boot.
This leads to a black screen lasting around 1-3 minutes following the boot loader message.
```

![Installation done](../../images/fusion-installdone.png)

Once installed, stop the virtual machine and edit its settings to disconnect the CD/DVD drive.

![Disconnect the CDROM](../../images/fusion-ejectdisk.png)

Start again and IncusOS will perform its first boot configuration and should startup successfully.

![Install updates](../../images/fusion-startup.png)

IncusOS will also install any pending updates.

![Install updates](../../images/fusion-updating.png)

## IncusOS is ready for use

The virtual machine should reboot and IncusOS would be ready for use.

Follow the instructions for [accessing the system](../access.md).

![Installed system](../../images/fusion-done.png)
