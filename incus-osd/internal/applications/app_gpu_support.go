package applications

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus/v7/shared/subprocess"
)

type gpuSupport struct {
	common
}

func (g *gpuSupport) Get(_ context.Context) (any, error) {
	return g.state.Applications.GPUSupport, nil
}

// IsInstalled reports whether the application has been installed.
func (g *gpuSupport) IsInstalled() bool {
	return isInstalled(g.Name(), g.appState.Version)
}

func (*gpuSupport) IsRunning(_ context.Context) bool {
	return true
}

func (*gpuSupport) Name() string {
	return "gpu-support"
}

// RequiresMatchingOSVersion reports if the application can only be used with the IncusOS version it was built with.
func (*gpuSupport) RequiresMatchingOSVersion() bool {
	// The application ships kernel modules which only work with the kernel they were built for.
	return true
}

// SetFriendlyVersion records the friendly version.
func (g *gpuSupport) SetFriendlyVersion(_ context.Context) error {
	version, err := os.ReadFile("/usr/lib/firmware/linux-firmware-gpu-version")
	if err != nil {
		return err
	}

	g.appState.FriendlyVersion = string(version) + " [" + g.appState.Version + "]"

	return nil
}

func (g *gpuSupport) Start(ctx context.Context) error {
	// Refresh the linker cache so the GPU libraries can be found.
	_, err := subprocess.RunCommandContext(ctx, "/sbin/ldconfig", "-X")
	if err != nil {
		slog.WarnContext(ctx, "Failed to refresh the linker cache", "err", err)
	}

	// Reload the modules if loaded.
	for _, module := range []string{"amdgpu", "i915", "intel_vpu", "nouveau", "xe"} {
		g.reloadModule(ctx, module)
	}

	// Load the drivers for any GPU that doesn't have one yet.
	nvidia := false

	for _, device := range g.unboundDevices() {
		switch {
		case device.vendor == "0x1002":
			g.loadModule(ctx, "amdgpu")
		case device.vendor == "0x10de":
			nvidia = true
		case device.vendor == "0x8086" && strings.HasPrefix(device.class, "0x03"):
			// Both drivers can be loaded, each only binds to the GPUs it supports by default.
			g.loadModule(ctx, "i915")
			g.loadModule(ctx, "xe")
		case device.vendor == "0x8086":
			g.loadModule(ctx, "intel_vpu")
		default:
		}
	}

	if nvidia {
		g.startNVIDIA(ctx)
	}

	return nil
}

// loadModule loads a kernel module unless it was blacklisted in the kernel configuration.
func (g *gpuSupport) loadModule(ctx context.Context, module string) {
	if slices.Contains(g.state.System.Kernel.Config.BlacklistModules, module) {
		return
	}

	_, err := subprocess.RunCommandContext(ctx, "/sbin/modprobe", module)
	if err != nil {
		slog.WarnContext(ctx, "Failed to load kernel module", "module", module, "err", err)
	}
}

// reloadModule reloads a loaded kernel module so it picks up newly available firmware.
func (*gpuSupport) reloadModule(ctx context.Context, module string) {
	// Check if loaded.
	_, err := os.Stat("/sys/module/" + module)
	if err != nil {
		return
	}

	// Unbind the devices.
	entries, err := os.ReadDir("/sys/bus/pci/drivers/" + module)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.WarnContext(ctx, "Failed to check kernel module", "module", module)

		return
	}

	for _, entry := range entries {
		address := filepath.Base(entry.Name())

		if !strings.Contains(address, ":") || !strings.Contains(address, ".") {
			continue
		}

		err := os.WriteFile("/sys/bus/pci/drivers/"+module+"/unbind", []byte(address), 0o600)
		if err != nil {
			slog.WarnContext(ctx, "Failed to unbind device", "module", module, "address", address)

			continue
		}
	}

	// Unload the module.
	_, err = subprocess.RunCommandContext(ctx, "/sbin/rmmod", module)
	if err != nil {
		slog.WarnContext(ctx, "Failed to unload kernel module", "module", module)

		return
	}

	// Load the module back.
	_, err = subprocess.RunCommandContext(ctx, "/sbin/modprobe", module)
	if err != nil {
		slog.WarnContext(ctx, "Failed to load kernel module", "module", module)
	}
}

// gpuDevice is a PCI GPU or compute accelerator.
type gpuDevice struct {
	class  string
	vendor string
}

// unboundDevices returns the GPUs and compute accelerators which don't have a driver.
func (*gpuSupport) unboundDevices() []gpuDevice {
	devices := []gpuDevice{}

	entries, err := os.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return devices
	}

	for _, entry := range entries {
		devicePath := filepath.Join("/sys/bus/pci/devices", entry.Name())

		// Only consider display controllers (0x03) and processing accelerators (0x12).
		class, err := os.ReadFile(filepath.Join(devicePath, "class"))
		if err != nil || (!strings.HasPrefix(string(class), "0x03") && !strings.HasPrefix(string(class), "0x12")) {
			continue
		}

		// Skip devices which already have a driver.
		_, err = os.Lstat(filepath.Join(devicePath, "driver"))
		if err == nil {
			continue
		}

		vendor, err := os.ReadFile(filepath.Join(devicePath, "vendor"))
		if err != nil {
			continue
		}

		devices = append(devices, gpuDevice{
			class:  strings.TrimSpace(string(class)),
			vendor: strings.TrimSpace(string(vendor)),
		})
	}

	return devices
}

// hasUnboundNVIDIA reports if an NVIDIA GPU is still without a driver.
func (g *gpuSupport) hasUnboundNVIDIA() bool {
	for _, device := range g.unboundDevices() {
		if device.vendor == "0x10de" {
			return true
		}
	}

	return false
}

// startNVIDIA loads the driver for NVIDIA GPUs.
func (g *gpuSupport) startNVIDIA(ctx context.Context) {
	// Both drivers match all NVIDIA GPUs, so prefer the NVIDIA one and leave what it doesn't support to nouveau.
	g.loadModule(ctx, "nvidia")

	if g.hasUnboundNVIDIA() {
		g.loadModule(ctx, "nouveau")
	}

	// Nothing else to do unless the NVIDIA driver is in use.
	_, err := os.Stat("/sys/bus/pci/drivers/nvidia")
	if err != nil {
		return
	}

	// Load the display module so the GPU gets a render node.
	g.loadModule(ctx, "nvidia_drm")

	// Load the compute module and create the device nodes that containers rely on.
	_, err = subprocess.RunCommandContext(ctx, "/usr/bin/nvidia-modprobe", "-u", "-c", "0")
	if err != nil {
		slog.WarnContext(ctx, "Failed to set up NVIDIA compute support", "err", err)
	}

	_, err = subprocess.RunCommandContext(ctx, "/usr/bin/nvidia-smi")
	if err != nil {
		slog.WarnContext(ctx, "Failed to query the NVIDIA GPUs", "err", err)
	}
}
