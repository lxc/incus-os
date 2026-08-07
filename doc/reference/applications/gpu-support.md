# GPU support

The GPU support (`gpu-support`) application includes the required firmware files for GPUs from one of:

* `amd`
* `nvidia`
* `intel`

```{note}
`gpu-support` only includes firmware files - it does not include any drivers (Kernel modules are always part of the IncusOS base image, never leaf images like `gpu-support`).
The out of tree Nvidia drivers are not available on an IncusOS host.
```
