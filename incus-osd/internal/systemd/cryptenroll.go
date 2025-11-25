package systemd

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/muesli/crunchy"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
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
	recoveryPassword, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--recovery-key", luksVolumes["root"])
	if err != nil {
		return err
	}

	recoveryPassword = strings.TrimSuffix(recoveryPassword, "\n")

	// Second, set the same recovery key for the swap volume. Need to pass to systemd-cryptenroll via NEWPASSWORD environment variable.
	_, _, err = subprocess.RunCommandSplit(ctx, append(os.Environ(), "NEWPASSWORD="+recoveryPassword), nil, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--password", luksVolumes["swap"])
	if err != nil {
		return err
	}

	// Finally, save the recovery key into the state.
	s.System.Security.Config.EncryptionRecoveryKeys = append(s.System.Security.Config.EncryptionRecoveryKeys, recoveryPassword)
	s.System.Security.State.EncryptionRecoveryKeysRetrieved = false

	return nil
}

// AddEncryptionKey utilizes systemd-cryptenroll to add a user-specified key for the
// root and swap LUKS volumes. Depends on an existing tpm2-backed key being enrolled and accessible.
func AddEncryptionKey(ctx context.Context, s *state.State, key string) error {
	if slices.Contains(s.System.Security.Config.EncryptionRecoveryKeys, key) {
		return errors.New("provided encryption key is already enrolled")
	}

	validator := crunchy.NewValidatorWithOpts(crunchy.Options{
		MinLength:         15,
		MustContainDigit:  false, // systemd-cryptenroll generates recovery passphrases that don't contain any digits.
		MustContainSymbol: true,
		CheckHIBP:         false,
	})

	err := validator.Check(key)
	if err != nil {
		return err
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

	s.System.Security.Config.EncryptionRecoveryKeys = append(s.System.Security.Config.EncryptionRecoveryKeys, key)

	return nil
}

// DeleteEncryptionKey utilizes systemd-cryptenroll to remove a user-specified key from the
// root and swap LUKS volumes. Depends on an existing tpm2-backed key being enrolled and accessible.
// Due to systemd-cryptenroll only being able to wipe slots by index or type, we must first
// remove all recovery and password slots, then re-add any remaining keys.
func DeleteEncryptionKey(ctx context.Context, s *state.State, key string) error {
	if !slices.Contains(s.System.Security.Config.EncryptionRecoveryKeys, key) {
		return errors.New("provided encryption key is not enrolled")
	}

	if len(s.System.Security.Config.EncryptionRecoveryKeys) == 1 {
		return errors.New("cannot remove only existing recovery key")
	}

	// Get the underlying LUKS partitions.
	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return err
	}

	// First, wipe all recovery and password slots.
	for _, volume := range luksVolumes {
		err := WipeAllRecoveryKeys(ctx, volume)
		if err != nil {
			return err
		}
	}

	existingKeys := s.System.Security.Config.EncryptionRecoveryKeys
	s.System.Security.Config.EncryptionRecoveryKeys = []string{}

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

// WipeAllRecoveryKeys will wipe all recovery and password key slots for the provided volume.
func WipeAllRecoveryKeys(ctx context.Context, volume string) error {
	_, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device", "auto", "--wipe-slot", "recovery,password", volume)

	return err
}

// ListEncryptedVolumes returns a list of each encrypted volume and its status.
func ListEncryptedVolumes(ctx context.Context) ([]api.SystemSecurityEncryptedVolume, error) {
	ret := []api.SystemSecurityEncryptedVolume{}

	// Get the LUKS partitions.
	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return ret, err
	}

	for volumeName, volumeDev := range luksVolumes {
		// First, check if the volume is mapped, and therefore unlocked.
		_, err := subprocess.RunCommandContext(ctx, "dmsetup", "info", volumeName)
		if err != nil {
			ret = append(ret, api.SystemSecurityEncryptedVolume{
				Volume: volumeName,
				State:  "locked",
			})

			continue
		}

		// Second, test if we can auto-unlock with the current TPM state.
		// Ideally we wouldn't have to depend on cryptsetup, but systemd-cryptenroll (and friends) don't
		// seem to have an equivalent of "--test-passphrase".
		_, err = subprocess.RunCommandContext(ctx, "cryptsetup", "luksOpen", "--test-passphrase", volumeDev, volumeName)
		if err != nil {
			// Do we have a PCR mismatch on the TPM? If so, assume we can unlock with the TPM upon reboot.
			if secureboot.TPMStatus() == secureboot.TPMPCRMismatch {
				ret = append(ret, api.SystemSecurityEncryptedVolume{
					Volume: volumeName,
					State:  "unlocked (TPM; PCR update pending)",
				})
			} else {
				ret = append(ret, api.SystemSecurityEncryptedVolume{
					Volume: volumeName,
					State:  "unlocked (recovery passphrase)",
				})
			}

			continue
		}

		// The volume auto-unlocked using the TPM.
		ret = append(ret, api.SystemSecurityEncryptedVolume{
			Volume: volumeName,
			State:  "unlocked (TPM)",
		})
	}

	return ret, nil
}
