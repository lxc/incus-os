# System requirements
IncusOS is designed to provide an extremely secure environment in which to
run Incus. It requires a lot of modern system features and will not function
properly on older unsupported systems.

Minimum system requirements:

- Modern Intel/AMD (`x86_64`) or ARM (`aarch64`) system
   - For `x86_64`, the CPU must support `x86_64_v3`
- Support for UEFI with Secure Boot
- {abbr}`TPM (Trusted Platform Module)` 2.0 security module
- At least 4GiB of RAM (for system use only)
- At least 50GiB of storage
- At least one wired network port

## Running in a degraded security state

```{warning}
Be aware that running IncusOS in a degraded security state will weaken the overall security of the IncusOS server, and is generally not supported in enterprise deployments.
```

For homelab and evaluation use, it is possible for IncusOS to run in a degraded security state with either:

- Secure Boot disabled
- A software-backed TPM

It is **NOT** possible for IncusOS to run with both Secure Boot disabled and a software-backed TPM.

Running IncusOS in either degraded security state presents a unique set of security trade-offs that are further documented in each reference page. Such systems may be treated differently by Operations Center or other products that interact with IncusOS via its API.

### Disabling Secure Boot

Certain physical servers and Microsoft Hyper-V have known incomplete and/or broken UEFI implementations that do not allow the enrollment of custom Secure Boot keys. To support these systems, it is possible to run IncusOS with Secure Boot disabled. For further details, see [Installing without Secure Boot](../reference/installing-without-secureboot.md).

### Using a software-backed TPM

Most consumer-grade ARM systems lack physical TPM chips, and it can be cost prohibitive to purchase one, such as when using a RaspberryPi. To support these systems, IncusOS can utilize `swtpm` for its TPM. For further details, see [Installing without a TPM](../reference/installing-without-tpm.md).
