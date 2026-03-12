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

// GenerateRecoveryKeys utilizes systemd-cryptenroll to generate recovery keys for the
// root and swap LUKS volumes. The logic depends on an existing tpm2-backed key being
// enrolled and accessible, which should be the case on first boot.
//
// A unique recovery key will be generated and saved to disk for each LUKS volume, which
// IncusOS will then use when performing future cryptsetup actions.
//
// If no encryption recovery keys currently exist in state, a second common recovery key
// will also be generated and reported via the security API. Each LUKS volume will enroll
// all encryption recovery keys from state in addition to the generated unique recovery key.
//
// This setup is required so IncusOS can always have a known-good recovery key even if
// the TPM is not in a state that can be used to unlock the LUKS volumes. systemd-cryptenroll
// differentiates between "recovery" and "password" passphrases. We rely on this in
// later code to distinguish between IncusOS (recovery) and user-provided (password)
// encryption keys.
func GenerateRecoveryKeys(ctx context.Context, s *state.State) error {
	// Get the underlying LUKS partitions.
	luksVolumes, err := util.GetLUKSVolumePartitions(ctx)
	if err != nil {
		return err
	}

	// On first boot, generate a common recovery key that will be used for both volumes and reported via the security API.
	if len(s.System.Security.Config.EncryptionRecoveryKeys) == 0 {
		commonRecoveryPassword, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device=auto", "--recovery-key", luksVolumes["root"])
		if err != nil {
			return err
		}

		commonRecoveryPassword = strings.TrimSuffix(commonRecoveryPassword, "\n")

		// Wipe that common recovery key, since we must manually enroll it on both volumes as a traditional "password" key.
		_, err = subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device=auto", "--wipe-slot=recovery", luksVolumes["root"])
		if err != nil {
			return err
		}

		// Save the new recovery key to state.
		s.System.Security.Config.EncryptionRecoveryKeys = append(s.System.Security.Config.EncryptionRecoveryKeys, commonRecoveryPassword)
		s.System.Security.State.EncryptionRecoveryKeysRetrieved = false
	}

	// Generate and save unique recovery keys for each volume.
	for volumeName, volumeDev := range luksVolumes {
		// Wipe any existing recovery or password keys.
		_, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device=auto", "--wipe-slot=recovery,password", volumeDev)
		if err != nil {
			return err
		}

		// Enroll a new recovery password.
		recoveryPassword, err := subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device=auto", "--recovery-key", volumeDev)
		if err != nil {
			return err
		}

		recoveryPassword = strings.TrimSuffix(recoveryPassword, "\n")

		// Write the recovery password to disk.
		f, err := os.Create("/var/lib/incus-os/recovery." + volumeName + ".key")
		if err != nil {
			return err
		}
		defer f.Close() //nolint:revive

		_, err = f.WriteString(recoveryPassword)
		if err != nil {
			return err
		}

		// Secure permissions on the file.
		err = f.Chmod(0o400)
		if err != nil {
			return err
		}
	}

	// Finally, enroll each encryption recovery key from state for each volume.
	for _, volumeDev := range luksVolumes {
		for _, key := range s.System.Security.Config.EncryptionRecoveryKeys {
			// Need to provide the recovery key to systemd-cryptenroll via the NEWPASSWORD environment variable.
			_, _, err := subprocess.RunCommandSplit(ctx, append(os.Environ(), "NEWPASSWORD="+key), nil, "systemd-cryptenroll", "--unlock-tpm2-device=auto", "--password", volumeDev)
			if err != nil {
				return err
			}
		}
	}

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
	luksVolumes, err := util.GetLUKSVolumePartitions(ctx)
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
	luksVolumes, err := util.GetLUKSVolumePartitions(ctx)
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
	luksVolumes, err := util.GetLUKSVolumePartitions(ctx)
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
