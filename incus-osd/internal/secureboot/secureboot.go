package secureboot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"debug/pe"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// Enabled checks if Secure Boot is currently enabled.
func Enabled() (bool, error) {
	state, err := readEFIVariable("SecureBoot")
	if err != nil {
		return false, err
	}

	return state[0] == 1, nil
}

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
	eventLog, err := readTPMEventLog()
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

	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return err
	}

	for _, volume := range luksVolumes {
		_, _, err := subprocess.RunCommandSplit(ctx, append(os.Environ(), "PASSWORD="+luksPassword), nil, "systemd-cryptenroll", "--tpm2-device=auto", "--wipe-slot=tpm2", "--tpm2-pcrlock=", "--tpm2-pcrs=7:sha256="+newPCR7String, volume)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdatePCR4Binding updates all LUKS encryption bindings to use the newly computed PCR4
// value in addition to PCR7 when the UKI is updated.
//
// Generally, this code should only be used when IncusOS is running with Secure Boot disabled
// and we rely on the additional binding of PCR4 in this degraded security state. Because PCR4
// is different for each UKI, this forces the use of a recovery passphrase if booting an older
// version of IncusOS, so use of PCR4 is limited just to instances with Secure Boot disabled.
func UpdatePCR4Binding(ctx context.Context, ukiFile string) error {
	// Verify the UKI file exists.
	_, err := os.Stat(ukiFile)
	if err != nil {
		return err
	}

	// Get and verify the current PCR states.
	eventLog, err := readTPMEventLog()
	if err != nil {
		return err
	}

	err = validateUntrustedTPMEventLog(eventLog)
	if err != nil {
		return err
	}

	// Compute new PCR4 value for the updated UKI.
	newPCR4, err := computeNewPCR4Value(eventLog, ukiFile)
	if err != nil {
		return err
	}

	// PCR7 won't change when the UKI is updated.
	pcr7, err := readPCR("7")
	if err != nil {
		return err
	}

	// Update the LUKS-encrypted volumes to use the new PCR4 value.
	newPCR4String := hex.EncodeToString(newPCR4)
	pcr7String := hex.EncodeToString(pcr7)

	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return err
	}

	for _, volume := range luksVolumes {
		_, err = subprocess.RunCommandContext(ctx, "systemd-cryptenroll", "--unlock-tpm2-device=auto", "--tpm2-device=auto", "--wipe-slot=tpm2", "--tpm2-pcrlock=", "--tpm2-pcrs=4:sha256="+newPCR4String+"+7:sha256="+pcr7String, volume)
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

// ListCertificates returns a list of all Secure Boot certificates present on the system.
func ListCertificates() []api.SystemSecuritySecureBootCertificate {
	ret := []api.SystemSecuritySecureBootCertificate{}

	for _, varName := range []string{"PK", "KEK", "db", "dbx"} {
		certs, err := GetCertificatesFromVar(varName)
		if err != nil {
			continue
		}

		for _, cert := range certs {
			rawFp := sha256.Sum256(cert.Raw)
			ret = append(ret, api.SystemSecuritySecureBootCertificate{
				Type:        varName,
				Fingerprint: hex.EncodeToString(rawFp[:]),
				Subject:     cert.Subject.String(),
				Issuer:      cert.Issuer.String(),
			})
		}
	}

	return ret
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

	dbCerts, err := GetCertificatesFromVar("db")
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

	dbxCerts, err := GetCertificatesFromVar("dbx")
	if err != nil {
		return err
	}

	if slices.ContainsFunc(dbxCerts, certEqualityFunc) {
		return errors.New("new UKI signed with revoked Secure Boot certificate, refusing to continue")
	}

	return nil
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
	efiFiles, err := getArchEFIFiles()
	if err != nil {
		return err
	}

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

	_, err = subprocess.RunCommandContext(ctx, "cp", filepath.Join(mountDir, efiFiles["stub"]), efiFiles["systemdEFI"])
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommandContext(ctx, "cp", filepath.Join(mountDir, efiFiles["stub"]), efiFiles["bootEFI"])
	if err != nil {
		return err
	}

	return nil
}

// getArchEFIFiles returns a map of architecture-specific file paths for the systemd-boot EFI stub
// and its installed locations.
func getArchEFIFiles() (map[string]string, error) {
	ret := make(map[string]string)

	switch runtime.GOARCH {
	case "amd64":
		ret["stub"] = "lib/systemd/boot/efi/systemd-bootx64.efi.signed"
		ret["systemdEFI"] = "/boot/EFI/systemd/systemd-bootx64.efi"
		ret["bootEFI"] = "/boot/EFI/BOOT/BOOTX64.EFI"

		return ret, nil
	case "arm64":
		ret["stub"] = "lib/systemd/boot/efi/systemd-bootaa64.efi.signed"
		ret["systemdEFI"] = "/boot/EFI/systemd/systemd-bootaa64.efi"
		ret["bootEFI"] = "/boot/EFI/BOOT/BOOTAA64.EFI"

		return ret, nil
	default:
		return ret, fmt.Errorf("architecture %s isn't currently supported", runtime.GOARCH)
	}
}
