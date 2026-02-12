package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// EncryptDrive wipes and formats a drive as a LUKS device.
func EncryptDrive(ctx context.Context, devPath string) error {
	if !strings.HasPrefix(devPath, "/dev/disk/by-id/") {
		return errors.New("invalid disk id")
	}

	devName := filepath.Base(devPath)
	keyfilePath := "/var/lib/incus-os/luks." + devName + ".key"

	// Wipe the drive.
	err := WipeDrive(ctx, devPath)
	if err != nil {
		return err
	}

	// Generate a random encryption key.
	err = util.GenerateEncryptionKeyFile(keyfilePath)
	if err != nil {
		return err
	}

	// Format the drive.
	_, err = subprocess.RunCommandContext(ctx, "cryptsetup", "luksFormat", "-q", devPath, keyfilePath)
	if err != nil {
		return err
	}

	// Unlock the drive.
	return unlockDrive(ctx, devPath)
}

// ImportEncryptedDrive decrypts a drive using a user provided key.
func ImportEncryptedDrive(ctx context.Context, devPath string, key string) error {
	if !strings.HasPrefix(devPath, "/dev/disk/by-id/") {
		return errors.New("invalid disk id")
	}

	devName := filepath.Base(devPath)
	keyfilePath := "/var/lib/incus-os/luks." + devName + ".key"

	// Decode encryption key into raw bytes.
	rawKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}

	if len(rawKey) != 32 {
		return fmt.Errorf("expected a 32 byte raw encryption key, got %d bytes", len(rawKey))
	}

	// Write the key to disk.
	err = os.WriteFile(keyfilePath, rawKey, 0o0600)
	if err != nil {
		return err
	}

	// Unlock the drive.
	err = unlockDrive(ctx, devPath)
	if err != nil {
		_ = os.Remove(keyfilePath)

		return err
	}

	return nil
}

// DecryptDrives decrypts all LUKS encrypted drives on the system where a key is available.
func DecryptDrives(ctx context.Context) error {
	// Look for keys.
	entries, err := os.ReadDir("/var/lib/incus-os")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		entryName := entry.Name()

		if !strings.HasPrefix(entryName, "luks.") || !strings.HasSuffix(entryName, ".key") {
			continue
		}

		devID := strings.TrimSuffix(strings.TrimPrefix(entryName, "luks."), ".key")
		devPath := "/dev/disk/by-id/" + devID

		_, err := os.Stat(devPath)
		if err != nil {
			slog.Warn("Couldn't find encrypted drive", "id", devPath, "err", err)

			continue
		}

		err = unlockDrive(ctx, devPath)
		if err != nil {
			slog.Warn("Couldn't unlock encrypted drive", "id", devPath, "err", err)

			continue
		}
	}

	return nil
}

// GetDriveKeys returns a map of device IDs to their base64 encoded keys.
func GetDriveKeys() (map[string]string, error) {
	keys := map[string]string{}

	// Look for keys.
	entries, err := os.ReadDir("/var/lib/incus-os")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		entryName := entry.Name()

		if !strings.HasPrefix(entryName, "luks.") || !strings.HasSuffix(entryName, ".key") {
			continue
		}

		devID := strings.TrimSuffix(strings.TrimPrefix(entryName, "luks."), ".key")

		devKey, err := os.ReadFile("/var/lib/incus-os/" + entryName) //nolint:gosec
		if err != nil {
			return nil, err
		}

		keys[devID] = base64.StdEncoding.EncodeToString(devKey)
	}

	return keys, nil
}

func unlockDrive(ctx context.Context, devPath string) error {
	devName := filepath.Base(devPath)
	keyfilePath := "/var/lib/incus-os/luks." + devName + ".key"

	_, err := os.Stat("/dev/mapper/luks-" + devName)
	if err == nil {
		// Already unlocked.
		return nil
	}

	_, err = subprocess.RunCommandContext(ctx, "cryptsetup", "open", "--type=luks", "-d", keyfilePath, devPath, "luks-"+devName)
	if err != nil {
		return err
	}

	return nil
}
