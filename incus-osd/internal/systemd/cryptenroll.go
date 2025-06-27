package systemd

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// GenerateRecoveryKey utilizes systemd-cryptenroll to generate a recovery key for the
// root and swap LUKS volumes. Depends on an existing tpm2-backed key being enrolled and accessible.
func GenerateRecoveryKey(ctx context.Context, s *state.State) error {
	// Get the underlying LUKS partitions.
	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return err
	}

	// First, generate a recovery key for the root volume.
	recoveryPassword, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--recovery-key", luksVolumes[0])
	if err != nil {
		return err
	}
	recoveryPassword = strings.TrimSuffix(recoveryPassword, "\n")

	// Second, set the same recovery key for the swap volume. Need to pass to systemd-cryptenroll via NEWPASSWORD environment variable.
	_, _, err = subprocess.RunCommandSplit(ctx, append(os.Environ(), "NEWPASSWORD="+recoveryPassword), nil, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--password", luksVolumes[1])
	if err != nil {
		return err
	}

	// Finally, save the recovery key into the state.
	s.System.Encryption.Config.RecoveryKeys = append(s.System.Encryption.Config.RecoveryKeys, recoveryPassword)
	s.System.Encryption.State.RecoveryKeysRetrieved = false

	return nil
}

// AddEncryptionKey utilizes systemd-cryptenroll to add a user-specified key for the
// root and swap LUKS volumes. Depends on an existing tpm2-backed key being enrolled and accessible.
func AddEncryptionKey(ctx context.Context, s *state.State, key string) error {
	if slices.Contains(s.System.Encryption.Config.RecoveryKeys, key) {
		return errors.New("provided encryption key is already enrolled")
	}

	// Get the underlying LUKS partitions.
	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return err
	}

	// Add the new encryption password. Need to pass to systemd-cryptenroll via NEWPASSWORD environment variable.
	for _, volume := range luksVolumes {
		_, _, err := subprocess.RunCommandSplit(ctx, append(os.Environ(), "NEWPASSWORD="+key), nil, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--password", volume)
		if err != nil {
			return err
		}
	}

	s.System.Encryption.Config.RecoveryKeys = append(s.System.Encryption.Config.RecoveryKeys, key)

	return nil
}

// DeleteEncryptionKey utilizes systemd-cryptenroll to remove a user-specified key from the
// root and swap LUKS volumes. Depends on an existing tpm2-backed key being enrolled and accessible.
// Due to systemd-cryptenroll only being able to wipe slots by index or type, we must first
// remove all recovery and password slots, then re-add any remaining keys.
func DeleteEncryptionKey(ctx context.Context, s *state.State, key string) error {
	if !slices.Contains(s.System.Encryption.Config.RecoveryKeys, key) {
		return errors.New("provided encryption key is not enrolled")
	}

	if len(s.System.Encryption.Config.RecoveryKeys) == 1 {
		return errors.New("cannot remove only existing recovery key")
	}

	// Get the underlying LUKS partitions.
	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return err
	}

	// First, wipe all recovery and password slots.
	for _, volume := range luksVolumes {
		_, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--wipe-slot", "recovery,password", volume)
		if err != nil {
			return err
		}
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

// SwapNeedsRecoveryKeySet checks if the swap partition has a recovery key set or not. This
// is needed to properly upgrade older installs of IncusOS that only have the recovery key
// set on the root partition. If the swap volume is missing the recovery key, attempts to update
// Secure Boot keys will fail.
//
// This should be removed by the end of August, 2025.
func SwapNeedsRecoveryKeySet(ctx context.Context) (bool, error) {
	output, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "/dev/disk/by-partlabel/swap")
	if err != nil {
		return false, err
	}

	return !strings.Contains(output, "password"), nil
}

// SwapSetRecoveryKey adds the specified recovery password to the swap volume.
//
// This should be removed by the end of August, 2025.
func SwapSetRecoveryKey(ctx context.Context, recoveryPassword string) error {
	// Need to pass to systemd-cryptenroll via NEWPASSWORD environment variable.
	_, _, err := subprocess.RunCommandSplit(ctx, append(os.Environ(), "NEWPASSWORD="+recoveryPassword), nil, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--password", "/dev/disk/by-partlabel/swap")

	return err
}
