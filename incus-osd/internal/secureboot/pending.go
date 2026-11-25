package secureboot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// ErrPendingUpdate is returned when an earlier EFI variable update is still awaiting confirmation.
var ErrPendingUpdate = errors.New("a Secure Boot variable update is still pending")

// VerifySecureBootUpdateApplied confirms whether the firmware kept the last EFI variable update across a reboot.
func VerifySecureBootUpdateApplied(ctx context.Context, s *state.State) error {
	pending := &s.SecureBoot.Pending

	if pending.Variable == "" {
		return nil
	}

	// The running firmware always shows the update until the system is rebooted.
	bootID, err := currentBootID()
	if err != nil {
		return err
	}

	if pending.BootID == bootID {
		return nil
	}

	present, err := variableHasCertificate(pending.Variable, pending.Fingerprint)
	if err != nil {
		return err
	}

	if !present {
		// Writing the variable again would only repeat the failure, so wait for a manual enrollment.
		// Both TPM bindings are kept so the system unlocks before and after it.
		pending.Dropped = true

		slog.WarnContext(ctx, "The firmware didn't keep a Secure Boot update, the certificate must be enrolled from the firmware setup", "variable", pending.Variable, "file", pending.KeyFile, "signature_guid", pending.OwnerGUID)

		return s.Save()
	}

	// Only trust the current PCR values if they were good enough for the TPM.
	tpmAutoUnlocked, err := util.TPMAutoUnlocked()
	if err != nil {
		return err
	}

	if !tpmAutoUnlocked {
		slog.WarnContext(ctx, "A Secure Boot update is present but doesn't match what the TPM expects, reset the TPM bindings to complete it", "variable", pending.Variable, "signature_guid", pending.OwnerGUID)

		return nil
	}

	// Drop the binding for the previous variable contents.
	err = bindCurrentPCRs(ctx)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "Secure Boot update confirmed", "variable", pending.Variable, "fingerprint", pending.Fingerprint)

	s.SecureBoot.Pending = state.SecureBootPending{}

	return s.Save()
}

// PendingEnrollment returns the certificate waiting to be enrolled from the firmware setup, if any.
func PendingEnrollment(s *state.State) *api.SystemSecuritySecureBootEnrollment {
	pending := s.SecureBoot.Pending

	if pending.Variable == "" || !pending.Dropped {
		return nil
	}

	return &api.SystemSecuritySecureBootEnrollment{
		Type:          pending.Variable,
		Fingerprint:   pending.Fingerprint,
		File:          pending.KeyFile,
		SignatureGUID: pending.OwnerGUID,
	}
}

// newPendingUpdate describes a signed EFI variable update and makes its certificate available on the ESP.
func newPendingUpdate(varName string, fingerprint []byte, authContents []byte) (*state.SecureBootPending, error) {
	certList, err := parseAuthCertificates(authContents)
	if err != nil {
		return nil, err
	}

	bootID, err := currentBootID()
	if err != nil {
		return nil, err
	}

	for _, entry := range certList {
		if entry.err != nil {
			continue
		}

		certFingerprint := sha256.Sum256(entry.cert.Raw)
		if !bytes.Equal(certFingerprint[:], fingerprint) {
			continue
		}

		name := espCertificateName(varName, entry.cert)
		if name == "" {
			name = "secureboot-" + varName + "-" + hex.EncodeToString(fingerprint[:4]) + ".der"
		}

		path := filepath.Join(espKeysPath, name)

		err = os.MkdirAll(espKeysPath, 0o755)
		if err != nil {
			return nil, err
		}

		err = os.WriteFile(path, entry.cert.Raw, 0o644) // #nosec G306
		if err != nil {
			return nil, err
		}

		return &state.SecureBootPending{
			Variable:    varName,
			Fingerprint: hex.EncodeToString(fingerprint),
			KeyFile:     path,
			OwnerGUID:   formatEFIGUID(entry.ownerGUID),
			BootID:      bootID,
		}, nil
	}

	return nil, fmt.Errorf("no certificate matching SHA256:%x in %s update", fingerprint, varName)
}

// variableHasCertificate checks the live EFI variable for a certificate.
func variableHasCertificate(varName string, fingerprint string) (bool, error) {
	val, err := util.ReadEFIVariable(varName)
	if err != nil {
		return false, err
	}

	certList, _, err := parseEfiSignatureList(val)
	if err != nil {
		return false, err
	}

	for _, entry := range certList {
		if entry.err != nil {
			continue
		}

		certFingerprint := sha256.Sum256(entry.cert.Raw)
		if hex.EncodeToString(certFingerprint[:]) == fingerprint {
			return true, nil
		}
	}

	return false, nil
}

// bindCurrentPCRs replaces all TPM bindings with one for the PCR values of the running system.
func bindCurrentPCRs(ctx context.Context) error {
	sbEnabled, err := Enabled()
	if err != nil {
		return err
	}

	eventLog, err := GetValidatedTPMEventLog()
	if err != nil {
		return err
	}

	pcr7, err := ReadPCR("7")
	if err != nil {
		return err
	}

	pcrBindingArg := pcrBindingArgument(nil, pcr7, nil)

	// When Secure Boot is disabled, we also bind to PCR4 and PCR12.
	if !sbEnabled {
		pcr4, err := ReadPCR("4")
		if err != nil {
			return err
		}

		pcr12, err := computeNewPCR12Value(eventLog)
		if err != nil {
			return err
		}

		pcrBindingArg = pcrBindingArgument(pcr4, pcr7, pcr12)
	}

	return enrollTPMBindings(ctx, pcrBindingArg, true)
}

// currentBootID returns the kernel's identifier for the current boot.
func currentBootID() (string, error) {
	content, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}

// formatEFIGUID returns the textual form of an EFI GUID, whose first three fields are little-endian.
func formatEFIGUID(guid [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%x-%x", binary.LittleEndian.Uint32(guid[0:4]), binary.LittleEndian.Uint16(guid[4:6]), binary.LittleEndian.Uint16(guid[6:8]), guid[8:10], guid[10:16])
}
