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

```{note}
For homelab and evaluation use, it is possible for IncusOS to rely on a software-backed TPM implementation. This is useful in scenarios such as running IncusOS on most consumer-grade ARM systems that may lack physical TPM chips.

Be aware that this will weaken the overall security of the IncusOS server, and is not supported in enterprise deployments. For further details, see [Installing without a TPM](../reference/installing-without-tpm.md).
```
