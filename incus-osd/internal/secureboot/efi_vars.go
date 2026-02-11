package secureboot

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	incusoscerts "github.com/lxc/incus-os/incus-osd/certs"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// GetCertificatesFromVar returns a list of certificates currently in a given EFI variable.
func GetCertificatesFromVar(varName string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate

	// Determine Secure Boot state.
	sbEnabled, err := Enabled()
	if err != nil {
		return nil, err
	}

	if sbEnabled {
		// In normal operation, Secure Boot will be enabled and we can
		// directly fetch certificates from a trusted EFI variable.
		val, err := readEFIVariable(varName)
		if err != nil {
			return nil, err
		}

		certList, err := parseEfiSignatureList(val)
		if err != nil {
			return nil, err
		}

		// Check for and report if any certificate failed to parse.
		for i, certInfo := range certList {
			if certInfo.err != nil {
				slog.Warn(fmt.Sprintf("Failed to parse Secure Boot variable '%s' certificate at index %d: %s", varName, i, certInfo.err.Error()))

				continue
			}

			certs = append(certs, certInfo.cert)
		}
	} else {
		// When Secure Boot is disabled, rely on any certificates baked into the IncusOS daemon.
		// Since the executable is part of the usr-verity image, it is read-only and the verity
		// image has been verified both during install/upgrade as well as boot-time checks against
		// the TPM event log. Therefore it should to be relatively safe to trust the contents.
		embeddedCerts, err := incusoscerts.GetEmbeddedCertificates()
		if err != nil {
			return nil, err
		}

		switch varName {
		case "PK":
			certs = []*x509.Certificate{embeddedCerts.SecureBootCertificates.PK}
		case "KEK":
			certs = embeddedCerts.SecureBootCertificates.KEK
		case "db":
			certs = embeddedCerts.SecureBootCertificates.DB
		case "dbx":
			certs = embeddedCerts.SecureBootCertificates.DBX
		default:
			return nil, errors.New("unable to get SecureBoot certificates from daemon for variable " + varName)
		}
	}

	// Some consumer-grade manufacturers ship invalid Secure Boot certificates. Unfortunately, sometimes
	// they are required for hardware such as storage or video cards to work. Since they are already in
	// the PK/KEK/db EFI variable we still process them, but log a warning each time we do.
	for _, cert := range certs {
		if cert.SerialNumber.Sign() < 0 {
			slog.Warn(fmt.Sprintf("Secure Boot variable '%s' contains invalid certificate '%s': serial number is negative", varName, cert.Subject))
		}

		switch publicKey := cert.PublicKey.(type) {
		case *rsa.PublicKey:
			if publicKey.Size()*8 > 2048 {
				slog.Warn(fmt.Sprintf("Secure Boot variable '%s' contains invalid certificate '%s': RSA key length %d is greater than 2048 bits", varName, cert.Subject, publicKey.Size()*8))
			}
		default:
			slog.Warn(fmt.Sprintf("Secure Boot variable '%s' contains invalid certificate '%s': expected a RSA key, got %s", varName, cert.Subject, cert.PublicKeyAlgorithm))
		}
	}

	return certs, nil
}

// UpdateSecureBootCerts takes a given tar archive and applies any SecureBoot KEK, db, or dbx
// updates that are not yet present on the current system.
func UpdateSecureBootCerts(ctx context.Context, tarArchive string) (bool, error) {
	kekUpdates := make(map[string][]byte)
	dbUpdates := make(map[string][]byte)
	dbxUpdates := make(map[string][]byte)

	// Determine Secure Boot state.
	sbEnabled, err := Enabled()
	if err != nil {
		return false, err
	}

	if !sbEnabled {
		return false, errors.New("Secure Boot is disabled, refusing to attempt a certificate update") //nolint:staticcheck
	}

	// #nosec G304
	archive, err := os.Open(tarArchive)
	if err != nil {
		return false, err
	}
	defer archive.Close()

	// Iterate through each update in the tar archive.
	tr := tar.NewReader(archive)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return false, err
		}

		// Only consider signed SecureBoot variable updates.
		if !strings.HasSuffix(header.Name, ".auth") {
			continue
		}

		if header.Size > 8192 {
			return false, fmt.Errorf("file '%s' is greater than 8192 bytes, rejecting update", header.Name)
		}

		// Filenames are of the format db_71CA141362BFE014F290119630C536451D575064C6336BEB0DF871F67E5323A8.auth.
		parts := strings.Split(header.Name, "_")
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid filename '%s', rejecting update", header.Name)
		}

		fingerprint := strings.TrimSuffix(parts[1], ".auth")
		buf := make([]byte, header.Size)

		n, err := tr.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		} else if int64(n) != header.Size {
			return false, fmt.Errorf("only read %d of %d bytes for file '%s'", n, header.Size, header.Name)
		}

		switch parts[0] {
		case "KEK":
			kekUpdates[fingerprint] = buf
		case "db":
			dbUpdates[fingerprint] = buf
		case "dbx":
			dbxUpdates[fingerprint] = buf
		default:
			return false, fmt.Errorf("unsupported SecureBoot variable update type '%s'", parts[0])
		}
	}

	// Apply any updates in order: KEK, then db, then dbx.
	needsReboot, err := applySecureBootUpdates(ctx, "KEK", kekUpdates)
	if err != nil {
		return needsReboot, err
	} else if needsReboot {
		return true, nil
	}

	needsReboot, err = applySecureBootUpdates(ctx, "db", dbUpdates)
	if err != nil {
		return needsReboot, err
	} else if needsReboot {
		return true, nil
	}

	needsReboot, err = applySecureBootUpdates(ctx, "dbx", dbxUpdates)
	if err != nil {
		return needsReboot, err
	} else if needsReboot {
		return true, nil
	}

	return false, nil
}

func applySecureBootUpdates(ctx context.Context, varName string, newCerts map[string][]byte) (bool, error) {
	existingCerts, err := GetCertificatesFromVar(varName)
	if err != nil {
		return false, fmt.Errorf("failed to read EFI variable %q: %w", varName, err)
	}

	for certFingerprint, certContents := range newCerts {
		certFingerprintBytes, err := hex.DecodeString(certFingerprint)
		if err != nil {
			return false, err
		}

		if slices.ContainsFunc(existingCerts, func(c *x509.Certificate) bool {
			cFingerprint := sha256.Sum256(c.Raw)

			return bytes.Equal(certFingerprintBytes, cFingerprint[:])
		}) {
			// This update is already present on the system, so nothing to do.
			continue
		}

		slog.InfoContext(ctx, "Appending certificate SHA256:"+certFingerprint+" to EFI variable "+varName)

		// Create a temp file for efi-updatevar to read from.
		f, err := os.CreateTemp("", "incus-os-sb-update")
		if err != nil {
			return false, err
		}
		defer os.Remove(f.Name()) //nolint:revive

		_, err = f.Write(certContents)
		if err != nil {
			return false, err
		}

		err = f.Close()
		if err != nil {
			return false, err
		}

		err = appendEFIVarUpdate(ctx, f.Name(), varName)
		if err != nil {
			if varName != "KEK" {
				return false, err
			}

			slog.WarnContext(ctx, "Failed to automatically apply KEK update, likely because a custom PK is configured")

			continue
		}

		slog.InfoContext(ctx, "Successfully updated EFI variable")

		// After applying a SecureBoot update, we need to restart before applying the next (if any).
		return true, nil
	}

	return false, nil
}

// appendEFIVarUpdate takes a pre-signed (.auth) EFI variable update, appends it
// to the current EFI value, and then updates the expected PCR7 value used to
// decrypt the root file system and swap at boot.
//
// NOTE: This function should never be called if Secure Boot is disabled, so the encryption
// binding logic doesn't include PCR4.
func appendEFIVarUpdate(ctx context.Context, efiUpdateFile string, varName string) error {
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
	eventLog, err := GetValidatedTPMEventLog()
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

	luksVolumes, err := util.GetLUKSVolumePartitions(ctx)
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

	buf, err := io.ReadAll(dbxFile)
	if err != nil {
		return err
	}

	// .auth files have a EFI_VARIABLE_AUTHENTICATION_2 header before the EFI signature list.
	// The first 16 bytes are EFI_TIME, followed by WIN_CERTIFICATE. The first four bytes of
	// WIN_CERTIFICATE are the total size of the header including the PKCS7 certificate data.
	// So the proper offset to the start of the EFI signature list is 16 + WIN_CERTIFICATE->dwLength.

	headerSize := binary.LittleEndian.Uint32(buf[16:20])
	offset := 16 + headerSize

	certList, err := parseEfiSignatureList(buf[offset:])
	if err != nil {
		return err
	} else if len(certList) != 1 {
		return fmt.Errorf("expected exactly one certificate in dbx update, got %d", len(certList))
	}

	if certList[0].err != nil {
		return errors.New("Failed to parse dbx certificate: " + certList[0].err.Error())
	}

	publicKeyDer, err := x509.MarshalPKIXPublicKey(certList[0].cert.PublicKey)
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
	case "LoaderEntrySelected":
		return "/sys/firmware/efi/efivars/LoaderEntrySelected-4a67b082-0a4c-41cf-b6c7-440b29bb8c4f", nil
	default:
		return "", fmt.Errorf("unsupported EFI variable '%s'", variableName)
	}
}
