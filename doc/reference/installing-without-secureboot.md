# Installing without Secure Boot

IncusOS depends on Secure Boot to prevent the booting of any non-IncusOS image and to provide the kernel a set of trusted certificates. However, a small number of systems have known incomplete and/or broken UEFI implementations that do not allow the enrollment of custom Secure Boot keys.

To support users who wish to run IncusOS on these broken systems, Secure Boot can be disabled. Please be aware that this will **WEAKEN THE OVERALL SECURITY OF THE INCUSOS SERVER**:

* The BIOS will allow any code to boot on the system. This might include malicious or attacker-controlled code.

* IncusOS will rely on the physical TPM PCR4 event log to verify that the `systemd-boot` stub and UKI that booted match the UEFI measurements and are signed by a trusted certificate. This is normally handled automatically when Secure Boot is enabled, but IncusOS will perform these actions early in the boot process. This opens some avenues of attack to an adversary with physical access to the system.

Additionally, the following limitations are imposed compared to normal operation:

* Because disk encryption is additionally bound to PCR4, when booting into a prior version of IncusOS you will always need to provide an encryption recovery passphrase.

* Trusted certificates are baked into the IncusOS image, rather than being updated via Secure Boot. This means that when a new Secure Boot key is rolled out IncusOS cannot use that key until an OS update is applied that includes it. This could potentially lead to upgrade issues if an IncusOS server is severely out of date and all available updates are signed by the new Secure Boot key.

Running IncusOS in an enterprise environment without Secure Boot is **NOT RECOMMENDED**.

IncusOS will display a prominent warning on its status screen when running without Secure Boot.

```{warning}
Here Be Dragons!

It must be reiterated that running IncusOS without Secure Boot weakens the overall security of the system and is not recommended for use in enterprise deployments.

Any IncusOS system that has ever booted with Secure Boot disabled will permanently record this fact and report it via the Security API `system_state_is_trusted` field. IncusOS systems that are not fully trusted may be treated differently by Operations Center or other products that interact with IncusOS via its API.
```

## Install seed

When configuring the IncusOS [install seed](./seed.md), set the `security.missing_secure_boot` field to `true`. This will allow IncusOS to boot with Secure Boot in a disabled state. This option will only take effect if Secure Boot is not enabled in the BIOS.

## Running off a live USB drive

If booting IncusOS off of a live USB drive and Secure Boot is disabled, IncusOS will display a 30 second security warning on first boot. Afterwards, the system will continue to boot as normal.
