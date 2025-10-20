package secureboot

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-eventlog/tcg"
	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// GetCertificatesFromVar returns a list of certificates currently in a given EFI variable.
func GetCertificatesFromVar(varName string) ([]x509.Certificate, error) {
	val, err := readEFIVariable(varName)
	if err != nil {
		return nil, err
	}

	parsedVal := tcg.UEFIVariableData{
		VariableData: val,
	}

	certs, _, err := parsedVal.SignatureData()

	return certs, err
}

// AppendEFIVarUpdate takes a pre-signed (.auth) EFI variable update, appends it
// to the current EFI value, and then updates the expected PCR7 value used to
// decrypt the root file system and swap at boot.
func AppendEFIVarUpdate(ctx context.Context, efiUpdateFile string, varName string) error {
	// Verify the file exists.
	_, err := os.Stat(efiUpdateFile)
	if err != nil {
		return err
	}

	// When updating the list of revoked certificates, ensure neither of the UKIs are signed
	// with it, otherwise we'd brick on a reboot with the affected UKI(s).
	if varName == "dbx" {
		err := checkDbxUpdateWouldBrickUKI(efiUpdateFile)
		if err != nil {
			return err
		}
	}

	// Get and verify the current PCR7 state.
	eventLog, err := readTMPEventLog()
	if err != nil {
		return err
	}

	err = validateUntrustedTPMEventLog(eventLog)
	if err != nil {
		return err
	}

	// By default, sysfs mounts EFI variables with the immutable attribute set. We need to remove it prior to appending the update.
	efiVarPath, err := efiVariableToFilename(varName)
	if err != nil {
		return err
	}

	_, err = os.Stat(efiVarPath)
	if err == nil {
		_, err = subprocess.RunCommandContext(ctx, "chattr", "-i", efiVarPath)
		if err != nil {
			return err
		}
	}

	// Apply the EFI variable update.
	_, err = subprocess.RunCommandContext(ctx, "efi-updatevar", "-a", "-f", efiUpdateFile, varName)
	if err != nil {
		if strings.Contains(err.Error(), "wrong filesystem permissions") {
			// Internally, if an EFI update doesn't apply (such as when signed by an untrusted certificate),
			// EACCES is returned and ultimately is reported as a file system error, which is a bit
			// confusing, so return a nicer error message.
			return fmt.Errorf("failed to apply %s update, likely due to a bad/untrusted signature", varName)
		}

		return err
	}

	// Compute the new expected PCR7 value on next boot.
	newPCR7, err := computeNewPCR7Value(eventLog)
	if err != nil {
		return err
	}

	// Update the LUKS-encrypted volumes to use the new PCR7 value.
	newPCR7String := hex.EncodeToString(newPCR7)

	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return err
	}

	for _, volume := range luksVolumes {
		_, err = subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device=auto", "--tpm2-device=auto", "--wipe-slot=tpm2", "--tpm2-pcrlock=", "--tpm2-pcrs=7:sha256="+newPCR7String, volume)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkDbxUpdateWouldBrickUKI checks if a proposed dbx update would invalidate a signed UKI
// currently present on the system, resulting in a bricked boot.
func checkDbxUpdateWouldBrickUKI(dbxFilePath string) error {
	// Get the pending dbx update certificate from signed .auth file.
	// #nosec G304
	dbxFile, err := os.Open(dbxFilePath)
	if err != nil {
		return err
	}
	defer dbxFile.Close()

	s, err := dbxFile.Stat()
	if err != nil {
		return err
	}

	// .auth files have a timestamp and AuthInfo header before the .esl content. For our use, skip 1287 bytes
	// into the .auth file to get actual certificate data.
	buf := make([]byte, s.Size()-1287)

	readBytes, err := dbxFile.ReadAt(buf, 1287)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	} else if readBytes != len(buf) {
		return fmt.Errorf("only read %d of %d expected bytes from EFI variable update '%s'", readBytes, len(buf), dbxFilePath)
	}

	efiVar := tcg.UEFIVariableData{
		VariableData: buf,
	}

	certs, _, err := efiVar.SignatureData()
	if err != nil {
		return err
	} else if len(certs) != 1 {
		return fmt.Errorf("expected exactly one certificate in dbx update, got %d", len(certs))
	}

	publicKeyDer, err := x509.MarshalPKIXPublicKey(certs[0].PublicKey)
	if err != nil {
		return err
	}

	publicKeyBlock := pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDer,
	}

	// Check each UKI image.
	ukis, err := os.ReadDir("/boot/EFI/Linux/")
	if err != nil {
		return err
	}

	for _, uki := range ukis {
		ukiFile := filepath.Join("/boot/EFI/Linux/", uki.Name())

		ukiPubKey, err := getPublicKeyFromUKI(ukiFile)
		if err != nil {
			return err
		}

		if bytes.Equal(pem.EncodeToMemory(&publicKeyBlock), ukiPubKey) {
			return fmt.Errorf("unable to apply dbx update, since UKI image '%s' is signed by the key which would be revoked", ukiFile)
		}
	}

	return nil
}

// readEFIVariable returns the current value, if any, of the specified EFI variable.
func readEFIVariable(variableName string) ([]byte, error) {
	// Determine which file to open.
	filename, err := efiVariableToFilename(variableName)
	if err != nil {
		return nil, err
	}

	// Open the file.
	// #nosec G304
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// If the EFI variable doesn't exist, return an empty buffer.
			return nil, nil
		}

		return nil, err
	}
	defer file.Close()

	// Get the contents of the EFI variable.
	s, err := file.Stat()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, s.Size())

	numBytes, err := io.ReadFull(file, buf)
	if err != nil {
		return nil, err
	} else if int64(numBytes) != s.Size() {
		return nil, fmt.Errorf("only read %d bytes from %s", numBytes, filename)
	}

	// Trim the first four bytes; https://docs.kernel.org/filesystems/efivarfs.html
	return buf[4:], nil
}

// efiVariableToFilename maps an EFI variable name to its file under /sys/.
func efiVariableToFilename(variableName string) (string, error) {
	switch variableName {
	case "SecureBoot":
		return "/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "SetupMode":
		return "/sys/firmware/efi/efivars/SetupMode-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "DeployedMode":
		return "/sys/firmware/efi/efivars/DeployedMode-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "AuditMode":
		return "/sys/firmware/efi/efivars/AuditMode-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "PK":
		return "/sys/firmware/efi/efivars/PK-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "KEK":
		return "/sys/firmware/efi/efivars/KEK-8be4df61-93ca-11d2-aa0d-00e098032b8c", nil
	case "db":
		return "/sys/firmware/efi/efivars/db-d719b2cb-3d3a-4596-a3bc-dad00e67656f", nil
	case "dbx":
		return "/sys/firmware/efi/efivars/dbx-d719b2cb-3d3a-4596-a3bc-dad00e67656f", nil
	default:
		return "", fmt.Errorf("unsupported EFI variable '%s'", variableName)
	}
}
