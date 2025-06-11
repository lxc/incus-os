package secureboot

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"slices"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
	"github.com/lxc/incus/v6/shared/subprocess"
)

// NOTE -- It's assumed that PCR7 is the only one we care about in this code.

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
func computeNewPCR7Value(ctx context.Context, eventLog []tcg.Event) ([]byte, error) {
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

				buf, err := computeExpectedVariableAuthority(ctx, e.Data)
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
func computeExpectedVariableAuthority(ctx context.Context, rawBuf []byte) ([]byte, error) {
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

	// Get signing information for systemd-boot EFI stub.
	output, err := subprocess.RunCommandContext(ctx, "sbverify", "--list", "/boot/EFI/BOOT/BOOTX64.EFI")
	if err != nil {
		return nil, err
	}

	subjectRegex := regexp.MustCompile(`subject: /CN=(.+)/O=(.+)`)
	issuerRegex := regexp.MustCompile(`issuer:  /CN=(.+)/O=(.+)`)

	subject := subjectRegex.FindStringSubmatch(output)
	issuer := issuerRegex.FindStringSubmatch(output)

	if len(subject) != 3 || len(issuer) != 3 {
		return nil, errors.New("failed to get certificate info for /boot/EFI/BOOT/BOOTX64.EFI")
	}

	// If the Issuer and Subject fields match, no need for further updates.
	// Ideally we'd use the certificate fingerprint, but I don't see any easy way
	// to get it from the EFI binary.
	if va.Certs[0].Issuer.String() == fmt.Sprintf("CN=%s,O=%s", issuer[1], issuer[2]) &&
		va.Certs[0].Subject.String() == fmt.Sprintf("CN=%s,O=%s", subject[1], subject[2]) {
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
		return c.Issuer.String() == fmt.Sprintf("CN=%s,O=%s", issuer[1], issuer[2]) &&
			c.Subject.String() == fmt.Sprintf("CN=%s,O=%s", subject[1], subject[2])
	})
	if index < 0 {
		return nil, fmt.Errorf("failed to find matching certificate for 'Issuer=%s; Subject=%s' in EFI db variable", fmt.Sprintf("CN=%s,O=%s", issuer[1], issuer[2]), fmt.Sprintf("CN=%s,O=%s", subject[1], subject[2]))
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
	var filename string
	switch variableName {
	case "SecureBoot":
		filename = "/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c"
	case "PK":
		filename = "/sys/firmware/efi/efivars/PK-8be4df61-93ca-11d2-aa0d-00e098032b8c"
	case "KEK":
		filename = "/sys/firmware/efi/efivars/KEK-8be4df61-93ca-11d2-aa0d-00e098032b8c"
	case "db":
		filename = "/sys/firmware/efi/efivars/db-d719b2cb-3d3a-4596-a3bc-dad00e67656f"
	case "dbx":
		filename = "/sys/firmware/efi/efivars/dbx-d719b2cb-3d3a-4596-a3bc-dad00e67656f"
	default:
		return nil, fmt.Errorf("unsupported EFI variable '%s'", variableName)
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
