package systemd

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// GenerateRecoveryKey utilizes systemd-cryptenroll to generate a recovery key for the
// root LUKS volume. Depends on an existing tpm2-backed key being enrolled and accessible.
func GenerateRecoveryKey(ctx context.Context, s *state.State) error {
	output, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--recovery-key", "/dev/disk/by-partlabel/root-x86-64")
	if err != nil {
		return err
	}

	s.System.Encryption.Config.RecoveryKeys = append(s.System.Encryption.Config.RecoveryKeys, strings.TrimSuffix(output, "\n"))
	s.System.Encryption.State.RecoveryKeysRetrieved = false

	return nil
}

// AddEncryptionKey utilizes systemd-cryptenroll to add a user-specified key for the
// root LUKS volume. Depends on an existing tpm2-backed key being enrolled and accessible.
func AddEncryptionKey(ctx context.Context, s *state.State, key string) error {
	if slices.Contains(s.System.Encryption.Config.RecoveryKeys, key) {
		return errors.New("provided encryption key is already enrolled")
	}

	// Add the new encryption password. Need to pass to systemd-cryptenroll via NEWPASSWORD environment variable.
	_, _, err := subprocess.RunCommandSplit(ctx, append(os.Environ(), "NEWPASSWORD="+key), nil, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--password", "/dev/disk/by-partlabel/root-x86-64")
	if err != nil {
		return err
	}

	s.System.Encryption.Config.RecoveryKeys = append(s.System.Encryption.Config.RecoveryKeys, key)

	return nil
}

// DeleteEncryptionKey utilizes systemd-cryptenroll to remove a user-specified key from the
// root LUKS volume. Depends on an existing tpm2-backed key being enrolled and accessible.
// Due to systemd-cryptenroll only being able to wipe slots by index or type, we must first
// remove all recovery and password slots, then re-add any remaining keys.
func DeleteEncryptionKey(ctx context.Context, s *state.State, key string) error {
	if !slices.Contains(s.System.Encryption.Config.RecoveryKeys, key) {
		return errors.New("provided encryption key is not enrolled")
	}

	// First, wipe all recovery and password slots.
	_, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--wipe-slot", "recovery,password", "/dev/disk/by-partlabel/root-x86-64")
	if err != nil {
		return err
	}

	existingKeys := s.System.Encryption.Config.RecoveryKeys
	s.System.Encryption.Config.RecoveryKeys = []string{}

	// Re-add remaining keys.
	for _, existingKey := range existingKeys {
		if existingKey == key {
			continue
		}

		err := AddEncryptionKey(ctx, s, existingKey)
		if err != nil {
			return err
		}
	}

	return nil
}
