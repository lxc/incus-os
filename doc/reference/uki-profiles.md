# UKI Profiles

IncusOS boots a default UKI profile that should work well across a wide variety of hardware. However, certain systems may benefit from booting an alternative UKI profile by selecting it from the boot menu. IncusOS will remember which UKI profile it last booted, and will attempt to boot into the same UKI profile on subsequent boots.

Currently, the following alternative UKI profiles are supported:

* `MacOS virtualization`: For IncusOS systems running on Apple's arm64 hardware, this will adjust the kernel's KVM configuration to improve the performance of virtual machines.

* `Display hardware info`: This is a debug option that will display information about currently detected hardware, which can be useful when creating an install seed or handling hardware changes. IncusOS will not actually boot when selecting this UKI profile.
