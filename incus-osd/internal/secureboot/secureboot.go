package secureboot

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"debug/pe"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/smallstep/pkcs7"
	"golang.org/x/sys/unix"
)

// NOTE -- It's assumed that PCR7 is the only one we care about in this code.

// HandleSecureBootKeyChange will apply the changes necessary when the Secure Boot
// signing key used for the UKIs is changed:
//
//	1: Verify the new certificate is in db and isn't in dbx.
//	2: Replace the existing systemd-boot EFI stub with the newly-signed one.
//	3: Compute the new PCR7 value expected on next boot.
//	4: Set the new Secure Boot public key to be used by the TPM for verifying
//	   the PCR11 policies. Since this will invalidate the current TPM state, we
//	   must have an alternative way of authenticating the LUKS changes; by
//	   default rely on the recovery passphrase that's automatically created on
//	   first boot.
func HandleSecureBootKeyChange(ctx context.Context, luksPassword string, ukiFile string, usrImageFile string) error {
	// Pre-checks -- Verify that the TPM event log matches current TPM values.
	eventLog, err := readTMPEventLog()
	if err != nil {
		return err
	}

	err = validateUntrustedTPMEventLog(eventLog)
	if err != nil {
		return err
	}

	// Part 1 -- Verify the new certificate is in db and isn't in dbx.
	newCert, err := getPublicKeyFromUKI(ukiFile)
	if err != nil {
		return err
	}

	err = validatePKICertificate(newCert)
	if err != nil {
		return err
	}

	// Part 2 -- Update the systemd-boot EFI stub.
	err = updateEFIBootStub(ctx, usrImageFile)
	if err != nil {
		return err
	}

	// Part 3 -- Compute the new PCR7 value.
	newPCR7, err := computeNewPCR7Value(eventLog)
	if err != nil {
		return err
	}

	// Part 4 -- Re-enroll the TPM utilizing the new Secure Boot public key.
	err = os.WriteFile("/run/systemd/tpm2-pcr-public-key.pem", newCert, 0o600)
	if err != nil {
		return err
	}

	newPCR7String := hex.EncodeToString(newPCR7)
	for _, volume := range []string{"/dev/disk/by-partlabel/root-x86-64", "/dev/disk/by-partlabel/swap"} {
		_, _, err := subprocess.RunCommandSplit(ctx, append(os.Environ(), "PASSWORD="+luksPassword), nil, "systemd-cryptenroll", "--tpm2-device=auto", "--wipe-slot=tpm2", "--tpm2-pcrlock=", "--tpm2-pcrs=7:sha256="+newPCR7String, volume)
		if err != nil {
			return err
		}
	}

	return nil
}

// UKIHasDifferentSecureBootCertificate returns a boolean indicating if a provided UKI is signed
// with a different Secure Boot certificate than the one that signed the currently running system.
func UKIHasDifferentSecureBootCertificate(ukiFile string) (bool, error) {
	currentCert := make([]byte, 451)
	file, err := os.Open("/run/systemd/tpm2-pcr-public-key.pem")
	if err != nil {
		return false, err
	}
	defer file.Close()

	count, err := file.Read(currentCert)
	if err != nil {
		return false, err
	} else if count != 451 {
		return false, fmt.Errorf("only read %d of 451 bytes while getting current public key from /run/systemd/tpm2-pcr-public-key.pem", count)
	}

	newCert, err := getPublicKeyFromUKI(ukiFile)
	if err != nil {
		return false, err
	}

	return !bytes.Equal(currentCert, newCert), nil
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

	// TODO -- when applying dbx, verify neither of the two images are relying on that certificate to boot.

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
		return err
	}

	// Compute the new expected PCR7 value on next boot.
	newPCR7, err := computeNewPCR7Value(eventLog)
	if err != nil {
		return err
	}

	// Update the LUKS-encrypted volumes to use the new PCR7 value.
	newPCR7String := hex.EncodeToString(newPCR7)
	for _, volume := range []string{"/dev/disk/by-partlabel/root-x86-64", "/dev/disk/by-partlabel/swap"} {
		_, err = subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device=auto", "--tpm2-device=auto", "--wipe-slot=tpm2", "--tpm2-pcrlock=", "--tpm2-pcrs=7:sha256="+newPCR7String, volume)
		if err != nil {
			return err
		}
	}

	return nil
}

// validatePKICertificate makes sure the certificate obtained from a potential new UKI
// is listed in the Secure Boot db, isn't in dbx, and is valid based on the current
// system time. (Secure Boot can't rely on time being correct; once up and running
// that's a reasonable assumption, but nothing security critical depends on this. Mostly
// it's just another easy check to help ensure we only install valid UKIs.)
func validatePKICertificate(cert []byte) error {
	certEqualityFunc := func(c x509.Certificate) bool {
		publicKeyDer, err := x509.MarshalPKIXPublicKey(c.PublicKey)
		if err != nil {
			return false
		}

		publicKeyBlock := pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyDer,
		}

		return bytes.Equal(pem.EncodeToMemory(&publicKeyBlock), cert)
	}

	dbVal, err := readEFIVariable("db")
	if err != nil {
		return err
	}

	db := tcg.UEFIVariableData{
		VariableData: dbVal,
	}

	dbCerts, _, err := db.SignatureData()
	if err != nil {
		return err
	}

	dbIndex := slices.IndexFunc(dbCerts, certEqualityFunc)

	if dbIndex < 0 {
		return errors.New("new UKI signed with certificate not present in Secure Boot db, refusing to continue")
	}

	if time.Now().Before(dbCerts[dbIndex].NotBefore) {
		return errors.New("new UKI signed with certificate that is not yet valid, refusing to continue")
	} else if time.Now().After(dbCerts[dbIndex].NotAfter) {
		return errors.New("new UKI signed with certificate that has expired, refusing to continue")
	}

	dbxVal, err := readEFIVariable("dbx")
	if err != nil {
		return err
	}

	dbx := tcg.UEFIVariableData{
		VariableData: dbxVal,
	}

	dbxCerts, _, err := dbx.SignatureData()
	if err != nil {
		return err
	}

	if slices.ContainsFunc(dbxCerts, certEqualityFunc) {
		return errors.New("new UKI signed with revoked Secure Boot certificate, refusing to continue")
	}

	return nil
}

// readTMPEventLog reads the raw TPM measurements and returns a parsed array of Events with SHA256 hashes.
func readTMPEventLog() ([]tcg.Event, error) {
	rawLog, err := os.Open("/sys/kernel/security/tpm0/binary_bios_measurements")
	if err != nil {
		return nil, err
	}
	defer rawLog.Close()

	buf, err := io.ReadAll(rawLog)
	if err != nil {
		return nil, err
	}

	log, err := tcg.ParseEventLog(buf, tcg.ParseOpts{})
	if err != nil {
		return nil, err
	}

	return log.Events(register.HashSHA256), nil
}

// validateUntrustedTPMEventLog takes an untrusted TPM event log and verifies if its values
// match what is currently reported by the TPM.
func validateUntrustedTPMEventLog(eventLog []tcg.Event) error {
	var err error

	// Playback the log and compute the resulting PCR7 value.
	untrustedPCR7Digest := make([]byte, 32)
	for _, e := range eventLog {
		if e.Index == 7 { // We only care about PCR7.
			untrustedPCR7Digest, err = extendPCRValue(untrustedPCR7Digest, e.ReplayedDigest(), false)
			if err != nil {
				return err
			}
		}
	}

	// Get the current PCR7 value from the TPM.
	pcr7File, err := os.Open("/sys/class/tpm/tpm0/pcr-sha256/7")
	if err != nil {
		return err
	}
	defer pcr7File.Close()

	actualPCR7Buf := make([]byte, 64)
	numBytes, err := io.ReadFull(pcr7File, actualPCR7Buf)
	if err != nil {
		return err
	} else if numBytes != 64 {
		return fmt.Errorf("only read %d bytes from /sys/class/tpm/tpm0/pcr-sha256/7", numBytes)
	}

	actualPCR7, err := hex.DecodeString(string(actualPCR7Buf))
	if err != nil {
		return err
	}

	if !bytes.Equal(actualPCR7, untrustedPCR7Digest) {
		return fmt.Errorf("computed PCR7 (%x) doesn't match actual value (%x)", untrustedPCR7Digest, actualPCR7)
	}

	return nil
}

// computeNewPCR7Value will compute the future PCR7 value after the KEK, db, and/or dbx EFI variables are updated.
// IMPORTANT: It is assumed that the provided TPM event log has already been validated.
func computeNewPCR7Value(eventLog []tcg.Event) ([]byte, error) {
	actualPCR7Buf := make([]byte, 32)

	for _, e := range eventLog {
		if e.Index == 7 { // We only care about PCR7.
			switch e.Type { //nolint:exhaustive
			case tcg.EFIVariableDriverConfig:
				// If an EFI variable (SecureBoot, PK, KEK, db, dbx), fetch the current value and use it for computing the PCR.

				buf, err := computeExpectedVariableDriverConfig(e.Data)
				if err != nil {
					return nil, err
				}

				actualPCR7Buf, err = extendPCRValue(actualPCR7Buf, buf, true)
				if err != nil {
					return nil, err
				}
			case tcg.EFIVariableAuthority:
				// Variable authority is the certificate used to sign EFI binaries (systemd-boot and the IncusOS image).
				// We expect the same certificate to be used for both; If there's a mis-match between the observed
				// certificate used for the systemd-boot EFI stub and the one in the event log, we are about to boot
				// with a new Secure Boot signing key. Fetch the expected new certificate from the EFI db variable
				// and use it for PCR7 computation.

				buf, err := computeExpectedVariableAuthority(e.Data)
				if err != nil {
					return nil, err
				}

				actualPCR7Buf, err = extendPCRValue(actualPCR7Buf, buf, true)
				if err != nil {
					return nil, err
				}
			default:
				// For all other types, re-use the existing digest from the event log.

				var err error
				actualPCR7Buf, err = extendPCRValue(actualPCR7Buf, e.ReplayedDigest(), false)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return actualPCR7Buf, nil
}

// computeExpectedVariableDriverConfig reads the current EFI variable, potentially updates the
// existing value, and returns an array of encoded bytes.
func computeExpectedVariableDriverConfig(rawBuf []byte) ([]byte, error) {
	v, err := tcg.ParseUEFIVariableData(bytes.NewReader(rawBuf))
	if err != nil {
		return nil, err
	}

	// Read the current variable.
	buf, err := readEFIVariable(v.VarName())
	if err != nil {
		return nil, err
	}

	// Update in-memory values.
	v.Header.VariableDataLength = uint64(len(buf))
	v.VariableData = buf

	// Get the updated buffer and use for PCR calculation.
	return v.Encode()
}

// computeExpectedVariableAuthority checks if the signature used by the systemd-boot EFI stub has
// changed, and if so, computes the new expected value.
func computeExpectedVariableAuthority(rawBuf []byte) ([]byte, error) {
	v, err := tcg.ParseUEFIVariableData(bytes.NewReader(rawBuf))
	if err != nil {
		return nil, err
	}

	va, err := tcg.ParseUEFIVariableAuthority(v)
	if err != nil {
		return nil, err
	}

	if len(va.Certs) != 1 {
		return nil, fmt.Errorf("expected exactly one certificate in VariableAuthority, got %d", len(va.Certs))
	}

	// Get existing certificate from systemd-boot EFI stub.
	binaryCert, err := extractCertificateFromPE("/boot/EFI/BOOT/BOOTX64.EFI")
	if err != nil {
		return nil, err
	}

	// If the certificates match, no need for further updates.
	if va.Certs[0].Equal(binaryCert) {
		return rawBuf, nil
	}

	// There was a mismatch. Try to get the expected certificate from the db.
	db, err := readEFIVariable("db")
	if err != nil {
		return nil, err
	}

	parsedDB := tcg.UEFIVariableData{
		VariableData: db,
	}

	certs, _, err := parsedDB.SignatureData()
	if err != nil {
		return nil, err
	}

	// Find the matching certificate.
	index := slices.IndexFunc(certs, func(c x509.Certificate) bool {
		return c.Equal(binaryCert)
	})
	if index < 0 {
		return nil, fmt.Errorf("failed to find matching certificate '%s' used by systemd-boot stub in EFI db variable", binaryCert.Subject.String())
	}

	// Update the variable's contents with the expected certificate value.
	var newBuf bytes.Buffer
	_, err = newBuf.Write(v.VariableData[:16]) // The first 16 bytes are a header; we shouldn't need to care about updating it since we're replacing a certificate with the same type/size as the existing one.
	if err != nil {
		return nil, err
	}
	_, err = newBuf.Write(certs[index].Raw)
	if err != nil {
		return nil, err
	}

	if newBuf.Len() != len(v.VariableData) {
		return nil, fmt.Errorf("resulting buffer size (%d) != expected size (%d)", newBuf.Len(), len(v.VariableData))
	}

	// Update in-memory values.
	v.VariableData = newBuf.Bytes()

	// Get the updated buffer and use for PCR calculation.
	return v.Encode()
}

// extractCertificateFromPE returns the signing certificate from a given PE binary.
// Adapted from https://github.com/doowon/sigtool/blob/master/sigtool.go, released under Apache-2.0 license.
func extractCertificateFromPE(filename string) (*x509.Certificate, error) {
	peFile, err := pe.Open(filename)
	if err != nil {
		return nil, err
	}
	defer peFile.Close()

	var certSize int64
	var offset int64

	switch t := peFile.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		certSize = int64(t.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY].Size) - 8
		offset = int64(t.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY].VirtualAddress) + 8
	default:
		return nil, fmt.Errorf("file '%s' doesn't appear to be a 64bit signed PE", filename)
	}

	if certSize <= -8 || offset <= 8 {
		return nil, fmt.Errorf("file '%s' doesn't appear to be a valid signed PE", filename)
	}

	// #nosec G304
	rawFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer rawFile.Close()

	buf := make([]byte, certSize)
	readBytes, err := rawFile.ReadAt(buf, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	} else if int64(readBytes) != certSize {
		return nil, fmt.Errorf("only read %d of %d expected bytes for certificate from PE '%s'", readBytes, certSize, filename)
	}

	pkcs, err := pkcs7.Parse(buf)
	if err != nil {
		return nil, err
	}

	if len(pkcs.Certificates) != 1 {
		return nil, fmt.Errorf("got %d certificates from PE '%s', expected exactly one", len(pkcs.Certificates), filename)
	}

	return pkcs.Certificates[0], nil
}

// extendPCRValue takes an existing pcr and extends it using the provided content.
func extendPCRValue(pcr []byte, content []byte, computeSHA256 bool) ([]byte, error) {
	hash := crypto.SHA256.New()
	_, err := hash.Write(pcr)
	if err != nil {
		return nil, err
	}

	if computeSHA256 {
		sum := sha256.Sum256(content)
		_, err := hash.Write(sum[:])
		if err != nil {
			return nil, err
		}
	} else {
		_, err := hash.Write(content)
		if err != nil {
			return nil, err
		}
	}

	return hash.Sum(nil), nil
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

// getPublicKeyFromUKI extracts the public key from a UKI image.
func getPublicKeyFromUKI(ukiFile string) ([]byte, error) {
	peFile, err := pe.Open(ukiFile)
	if err != nil {
		return nil, err
	}
	defer peFile.Close()

	pcrpkeySection := peFile.Section(".pcrpkey")
	if pcrpkeySection == nil {
		return nil, fmt.Errorf("failed to read .pcrpkey section from '%s'", ukiFile)
	}

	pcrpkeyData, err := pcrpkeySection.Data()
	if err != nil {
		return nil, err
	} else if len(pcrpkeyData) != 512 {
		return nil, fmt.Errorf("only read %d of 512 bytes while getting UKI public key from '%s'", len(pcrpkeyData), ukiFile)
	}

	// Trim null bytes from returned buffer.
	return pcrpkeyData[:451], nil
}

// updateEFIBootStub synchronizes the systemd-boot EFI stub when the Secure Boot signing key is rotated.
func updateEFIBootStub(ctx context.Context, usrImageFile string) error {
	mountDir, err := os.MkdirTemp("/tmp", "incus-os")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mountDir)

	err = unix.Mount(usrImageFile, mountDir, "erofs", 0, "ro")
	if err != nil {
		return err
	}
	defer unix.Unmount(mountDir, 0)

	_, err = subprocess.RunCommandContext(ctx, "cp", filepath.Join(mountDir, "lib/systemd/boot/efi/systemd-bootx64.efi.signed"), "/boot/EFI/systemd/systemd-bootx64.efi")
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommandContext(ctx, "cp", filepath.Join(mountDir, "lib/systemd/boot/efi/systemd-bootx64.efi.signed"), "/boot/EFI/BOOT/BOOTX64.EFI")
	if err != nil {
		return err
	}

	return nil
}
