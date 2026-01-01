package util

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"
)

// UpdateSystemCustomCACerts regenerates /etc/ssl/certs/ca-certificates.crt with any
// specified custom CA certificates appended.
func UpdateSystemCustomCACerts(pemCerts []string) error {
	// If no custom CA certificates are configured, restore the default symlink.
	if len(pemCerts) == 0 {
		// Remove any existing locally-configured certificates.
		err := os.RemoveAll("/etc/ssl/certs/")
		if err != nil {
			return err
		}

		return os.Symlink("/usr/share/certs", "/etc/ssl/certs")
	}

	// Validate that each PEM-encoded block is a valid x509 certificate.
	for i, pemCert := range pemCerts {
		pemBlock, _ := pem.Decode([]byte(pemCert))
		if pemBlock == nil {
			return fmt.Errorf("unable to decode certificate %d", i)
		}

		if pemBlock.Type != "CERTIFICATE" {
			return fmt.Errorf("certificate %d isn't a PEM-encoded certificate", i)
		}

		_, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return err
		}
	}

	// Remove any existing locally-configured certificates.
	err := os.RemoveAll("/etc/ssl/certs/")
	if err != nil {
		return err
	}

	// Create an empty directory.
	err = os.MkdirAll("/etc/ssl/certs/", 0o755)
	if err != nil {
		return err
	}

	// Create the new CA bundle.
	bundleCerts, err := os.Create("/etc/ssl/certs/ca-certificates.crt")
	if err != nil {
		return err
	}
	defer bundleCerts.Close()

	// Get default CA certificates from the verity image.
	systemCerts, err := os.Open("/usr/share/certs/ca-certificates.crt")
	if err != nil {
		return err
	}
	defer systemCerts.Close()

	// Copy the default CA certificates. Do it in a single call, since the file is only ~220KB.
	_, err = io.Copy(bundleCerts, systemCerts)
	if err != nil {
		return err
	}

	// Append each custom CA certificate.
	for _, pemCert := range pemCerts {
		_, err := bundleCerts.WriteString(pemCert)
		if err != nil {
			return err
		}

		// Ensure we end with a newline, even if the PEM-encoded string didn't have one.
		if !strings.HasSuffix(pemCert, "\n") {
			_, err := bundleCerts.WriteString("\n")
			if err != nil {
				return err
			}
		}
	}

	return nil
}
