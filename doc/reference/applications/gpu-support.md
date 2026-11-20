# GPU support

The GPU support (`gpu-support`) application includes the kernel drivers, firmware and tools needed to use GPUs from one of:

* `amd`
* `nvidia`
* `intel`

Without it, IncusOS only includes the basic display drivers needed for a local console.

## Kernel drivers

Kernel drivers only work with the exact kernel they were built for. Because of that, `gpu-support` behaves differently
from other applications and always uses the version matching the running IncusOS version.

When an update is available, the new version of `gpu-support` gets downloaded along with the IncusOS update, but only
becomes active once the system is rebooted into the new IncusOS version.

```{note}
When installing `gpu-support` on a system that isn't running the latest IncusOS version, both the version matching
the running system and the latest version get downloaded. If the version matching the running system is no longer
available, the installation fails and the system needs to be updated and rebooted first.
```

## NVIDIA

For NVIDIA GPUs, the application includes both the open source `nouveau` driver and the
[NVIDIA open GPU kernel modules](https://github.com/NVIDIA/open-gpu-kernel-modules), along with the firmware,
tools (`nvidia-smi`) and libraries needed for compute (CUDA, OpenCL), video encoding or decoding and
headless rendering (EGL, OpenGL ES).
This allows the use of `nvidia.runtime=true` on Incus containers and lets Incus render on the GPU on behalf of virtual machines.

The NVIDIA driver version follows the most recent production or long term support branch from the
[NVIDIA data center driver releases](https://docs.nvidia.com/datacenter/tesla/drivers/index.html).

IncusOS will use the NVIDIA driver for every GPU that it supports and fall back to `nouveau` for older GPUs.
To always use `nouveau`, add `nvidia` to the list of [blacklisted kernel modules](../system/kernel.md).

The libraries tied to a display server (GLX, X11, Wayland) and Vulkan aren't included.

```{note}
The NVIDIA firmware, tools and libraries are proprietary software distributed under the terms of the NVIDIA license.
The full text of that license is available by running `incus admin os license show local nvidia-driver`.
```
