package cli

import (
	"fmt"
	"os"
	"strings"

	incusapi "github.com/lxc/incus/v7/shared/api"
	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/lxc/incus/v7/shared/units"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// renderStorageInfo decodes a system/storage response and renders drive and pool tables.
func renderStorageInfo(resp *incusapi.Response) error {
	var storage api.SystemStorage

	err := resp.MetadataAsStruct(&storage)
	if err != nil {
		return err
	}

	// Drives table.
	fmt.Println("Drives:") //nolint:forbidigo

	driveRows := make([][]string, 0, len(storage.State.Drives))
	for _, drive := range storage.State.Drives {
		boot := ""
		if drive.Boot {
			boot = "yes"
		}

		encrypted := ""
		if drive.Encrypted {
			encrypted = "yes"
		}

		driveRows = append(driveRows, []string{
			drive.ID,
			drive.ModelName,
			drive.SerialNumber,
			drive.Bus,
			units.GetByteSizeStringIEC(int64(drive.CapacityInBytes), 2), //nolint:gosec
			boot,
			encrypted,
			drive.MemberPool,
		})
	}

	driveHeader := []string{"ID", "MODEL", "SERIAL", "BUS", "CAPACITY", "BOOT", "ENCRYPTED", "MEMBER POOL"}

	err = cli.RenderTable(os.Stdout, "table", driveHeader, driveRows, nil)
	if err != nil {
		return err
	}

	// Pools table.
	if len(storage.State.Pools) > 0 {
		fmt.Println("\nPools:") //nolint:forbidigo

		poolRows := make([][]string, 0, len(storage.State.Pools))
		for _, pool := range storage.State.Pools {
			poolRows = append(poolRows, []string{
				pool.Name,
				pool.Type,
				pool.State,
				pool.EncryptionKeyStatus,
				units.GetByteSizeStringIEC(int64(pool.RawPoolSizeInBytes), 2),        //nolint:gosec
				units.GetByteSizeStringIEC(int64(pool.UsablePoolSizeInBytes), 2),     //nolint:gosec
				units.GetByteSizeStringIEC(int64(pool.PoolAllocatedSpaceInBytes), 2), //nolint:gosec
				strings.Join(pool.Devices, "\n"),
			})
		}

		poolHeader := []string{"NAME", "TYPE", "STATE", "ENCRYPTION KEY", "RAW SIZE", "USABLE SIZE", "ALLOCATED", "DEVICES"}

		return cli.RenderTable(os.Stdout, "table", poolHeader, poolRows, nil)
	}

	return nil
}

// systemInfoStorageCommand returns an info command for the system/storage endpoint.
func systemInfoStorageCommand(c *cmdAdminOS, endpoint string, description string) *cobra.Command {
	return (&cmdGenericInfo{
		os:          c,
		endpoint:    endpoint,
		description: description,
		handler:     renderStorageInfo,
	}).command()
}
