package systemd

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/smallstep/pkcs7"

	"github.com/lxc/incus-os/incus-osd/internal/keyring"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
)

type veritySignatureMetadata struct {
	RootHash               string `json:"rootHash"`               //nolint:tagliatelle
	CertificateFingerprint string `json:"certificateFingerprint"` //nolint:tagliatelle
	Signature              string `json:"signature"`
}

func getTrustedVerityCertificate(ctx context.Context, certificateFingerprint string) (*x509.Certificate, error) {
	// Get db Secure Boot certificates.
	certs, err := secureboot.GetCertificatesFromVar("db")
	if err != nil {
		return nil, err
	}

	// Get kernel's trusted platform keys.
	kernelKeys, err := keyring.GetKeys(ctx, keyring.PlatformKeyring)
	if err != nil {
		return nil, err
	}

	// Iterate through Secure Boot certificates to find a match.
	for _, cert := range certs {
		sha256Fp := sha256.Sum256(cert.Raw)

		// The image fingerprint matches a certificate in Secure Boot db.
		if certificateFingerprint == hex.EncodeToString(sha256Fp[:]) {
			// Iterate through kernel trusted keys to find a match.
			for _, key := range kernelKeys {
				// Return the trusted certificate that matches the verity fingerprint.
				if key.Fingerprint == hex.EncodeToString(cert.SubjectKeyId) {
					return cert, nil
				}
			}

			return nil, errors.New("verity image is signed by a trusted Secure Boot certificate, but the certificate isn't present in the kernel's keyring (reboot needed?)")
		}
	}

	return nil, errors.New("verity image is not signed by a trusted certificate")
}

func verifySignature(pkcsBase64 string, rootHash string, trustedCert *x509.Certificate) error {
	pkcsBytes, err := base64.StdEncoding.DecodeString(pkcsBase64)
	if err != nil {
		return err
	}

	pkcs, err := pkcs7.Parse(pkcsBytes)
	if err != nil {
		return err
	}

	if len(pkcs.Certificates) != 1 {
		return errors.New("expected exactly one signing certificate")
	}

	// Overwrite whatever certificate might be in the PKCS7 with the one we know
	// is trusted. This prevents an attacker from providing forged metadata with a
	// correct fingerprint, but wrong signing certificate. We could setup a full CA
	// certificate chain to use when verifying, but the certificates from the kernel's
	// keyring are already vetted.
	pkcs.Certificates[0] = trustedCert

	// Add content for the detached PKCS7 signature.
	pkcs.Content = []byte(rootHash)

	return pkcs.Verify()
}
