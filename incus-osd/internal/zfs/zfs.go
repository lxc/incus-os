package zfs

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
)

var zfsLocalKeyfile = "/var/lib/incus-os/zpool.local.key"

// ImportOrCreateLocalPool imports and loads the encryption key for the "local" ZFS pool if the it
// exists, otherwise will create an encrypted ZFS pool "local" in the partition labeled "local-data".
func ImportOrCreateLocalPool(ctx context.Context) error {
	// Check if the "local" ZFS pool exists.
	_, err := subprocess.RunCommandContext(ctx, "zpool", "import", "local")
	if err == nil || strings.Contains(err.Error(), "cannot import 'local': a pool with that name already exists") {
		// Pool is available, now load the encryption key and we're done.
		_, err := subprocess.RunCommandContext(ctx, "zfs", "load-key", "local")
		if err != nil && !strings.Contains(err.Error(), "Key load error: Key already loaded for 'local'.") {
			return err
		}

		return nil
	} else if strings.Contains(err.Error(), "cannot import 'local': no such pool available") {
		// Need to create the "local" ZFS pool.

		// Create a random encryption key file.
		{
			devUrandom, err := os.OpenFile("/dev/urandom", os.O_RDONLY, 0o0600)
			if err != nil {
				return err
			}
			defer devUrandom.Close()

			keyfile, err := os.OpenFile(zfsLocalKeyfile, os.O_CREATE|os.O_WRONLY, 0o0600)
			if err != nil {
				return err
			}
			defer keyfile.Close()

			count, err := io.CopyN(keyfile, devUrandom, 32)
			if err != nil {
				return err
			}

			if count != 32 {
				return errors.New("failed to read 32 bytes from /dev/urandom")
			}
		}

		// Create the ZFS pool.
		_, err = subprocess.RunCommandContext(ctx, "zpool", "create", "-o", "ashift=12", "-O", "mountpoint=none", "-O", "encryption=aes-256-gcm", "-O", "keyformat=raw", "-O", "keylocation=file://"+zfsLocalKeyfile, "local", "/dev/disk/by-partlabel/local-data")

		return err
	}

	return err
}
