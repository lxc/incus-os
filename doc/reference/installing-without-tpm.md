# Installing without a TPM

IncusOS depends on the TPM in conjunction with Secure Boot to provide trusted measurements of the system state when applying updates and automatically unlocking encrypted storage pools at boot. However, not all hardware systems come with a TPM, and for those that don't it may be cost prohibitive to purchase a TPM. A common example are consumer-grade ARM systems, such as the RaspberryPi.

To support homelab users and others who wish to run IncusOS on physical machines without a TPM, a [`swtpm`-backed](https://github.com/stefanberger/swtpm) TPM can be configured. Please be aware that this will **WEAKEN THE OVERALL SECURITY OF THE INCUSOS SERVER**:

* Unlike a physical TPM, it is trivial to inspect and change the software TPM state.

* A physical TPM has already measured several critical PCR values before the kernel starts booting. IncusOS performs these same measurements early in the boot process, but from user-space. This opens some avenues of attack, but is at least partially mitigated by the Secure Boot configuration.

Running IncusOS in an enterprise environment without a physical TPM is **UNSUPPORTED**.

IncusOS will display a prominent warning on its status screen when running with a software-backed TPM.

```{warning}
Here Be Dragons!

It must be reiterated that running IncusOS without a physical TPM weakens the overall security of the system and is not intended for use in enterprise deployments.

Any IncusOS system that has ever booted with a software-backed TPM will permanently record this fact and report it via the Security API `system_state_is_trusted` field. IncusOS systems that are not fully trusted may be treated differently by Operations Center or other products that interact with IncusOS via its API.
```

## Install seed

When configuring the IncusOS [install seed](./seed.md), set the `use_swtpm` field to `true`. This will cause IncusOS to configure a software TPM during installation. This option will only take affect if no physical TPM is detected.

## Running off a live USB drive

If booting IncusOS off of a live USB drive, IncusOS will automatically configure a software TPM on first boot if no physical TPM is detected and then automatically restart. This restart is necessary to properly initialize the software TPM and setting up the encrypted disk partitions for use.
