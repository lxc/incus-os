package cli

import (
	"fmt"

	incusapi "github.com/lxc/incus/v7/shared/api"
	"github.com/lxc/incus/v7/shared/units"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// renderKernelInfo decodes a system/kernel response and renders its state.
func renderKernelInfo(resp *incusapi.Response) error {
	var kernel api.SystemKernel

	err := resp.MetadataAsStruct(&kernel)
	if err != nil {
		return err
	}

	if kernel.State.Memory == nil || kernel.State.Memory.ZramSwap == nil {
		_, _ = fmt.Println("No zram-backed swap device configured") //nolint:forbidigo

		return nil
	}

	swap := kernel.State.Memory.ZramSwap

	_, _ = fmt.Print("Zram swap:\n")                                                                            //nolint:forbidigo
	_, _ = fmt.Printf("  Disk size: %s\n", units.GetByteSizeStringIEC(int64(swap.Disksize), 2))                 //nolint:forbidigo
	_, _ = fmt.Printf("  Uncompressed data: %s\n", units.GetByteSizeStringIEC(int64(swap.UncompressedSize), 2)) //nolint:forbidigo
	_, _ = fmt.Printf("  Compressed data: %s\n", units.GetByteSizeStringIEC(int64(swap.CompressedSize), 2))     //nolint:forbidigo
	_, _ = fmt.Printf("  Compression ratio: %.2f\n", swap.CompressionRatio)                                     //nolint:forbidigo
	_, _ = fmt.Printf("  Total memory used: %s\n", units.GetByteSizeStringIEC(int64(swap.TotalMemoryUse), 2))   //nolint:forbidigo

	return nil
}

// systemInfoKernelCommand returns an info command for the system/kernel endpoint.
func systemInfoKernelCommand(c *cmdAdminOS, endpoint string, description string) *cobra.Command {
	return (&cmdGenericInfo{
		os:          c,
		endpoint:    endpoint,
		description: description,
		handler:     renderKernelInfo,
	}).command()
}
