package kernel

import (
	"context"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/lxc/incus/v7/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
)

// GetDebugInfo returns kernel debug information about the running system.
func GetDebugInfo(ctx context.Context) (*api.DebugKernel, error) {
	info := &api.DebugKernel{}

	// Retrieve the kernel release and architecture.
	uts := unix.Utsname{}

	err := unix.Uname(&uts)
	if err != nil {
		return nil, err
	}

	info.Version = unix.ByteSliceToString(uts.Release[:])
	info.Architecture = unix.ByteSliceToString(uts.Machine[:])

	// Retrieve the CPU baseline.
	info.CPUBaseline = cpuBaseline(ctx)

	// Retrieve the module list.
	info.Modules, err = getModules()
	if err != nil {
		return nil, err
	}

	return info, nil
}

// cpuBaseline determines the x86_64 micro-architecture baseline from the dynamic loader.
func cpuBaseline(ctx context.Context) string {
	output, err := subprocess.RunCommandContext(ctx, "/lib64/ld-linux-x86-64.so.2", "--help")
	if err != nil {
		return ""
	}

	// The loader lists the glibc-hwcaps subdirectories in priority order,
	// the first supported entry is the system's baseline.
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "x86-64-v") && strings.HasPrefix(fields[1], "(supported") {
			return strings.ReplaceAll(fields[0], "-", "_")
		}
	}

	// Successfully running the x86-64 loader implies the base level.
	return "x86_64_v1"
}

// getModules returns the list of loaded kernel modules with their dependencies.
func getModules() ([]api.DebugKernelModule, error) {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return nil, err
	}

	dependencies := map[string][]string{}
	inUse := map[string]bool{}

	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		name := fields[0]

		_, ok := dependencies[name]
		if !ok {
			dependencies[name] = []string{}
		}

		// The third field holds the module's reference count.
		inUse[name] = fields[2] != "0"

		// The fourth field holds the list of modules that depend on this one.
		if fields[3] == "-" {
			continue
		}

		for user := range strings.SplitSeq(strings.TrimSuffix(fields[3], ","), ",") {
			dependencies[user] = append(dependencies[user], name)
		}
	}

	modules := make([]api.DebugKernelModule, 0, len(dependencies))

	for _, name := range slices.Sorted(maps.Keys(dependencies)) {
		deps := dependencies[name]
		slices.Sort(deps)

		modules = append(modules, api.DebugKernelModule{
			Name:         name,
			Dependencies: deps,
			InUse:        inUse[name],
		})
	}

	return modules, nil
}
