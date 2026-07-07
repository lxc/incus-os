package util

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/lxc/incus/v7/shared/subprocess"
	"github.com/smallstep/pkcs7"

	"github.com/lxc/incus-os/incus-osd/certs"
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

// VerifySMIME first verifies the provided S/MIME-signed message using the root CA as a trust
// anchor. Then, if validation succeeded, it ensures one of the expected intermediate CAs is
// present in the provided certificate chain. `openssl smime ...` doesn't provide a way to do
// this, so we must perform a manual check ourselves. Upon success, the verified plain-text
// message is returned.
func VerifySMIME(ctx context.Context, expectedIntermediateCAs []*x509.Certificate, message []byte) (*bytes.Buffer, error) {
	if len(expectedIntermediateCAs) == 0 {
		return nil, errors.New("at least one intermediate CA must be provided when performing S/MIME validation")
	}

	for _, ca := range expectedIntermediateCAs {
		if ca == nil {
			return nil, errors.New("provided intermediate CA for S/MIME validation cannot be nil")
		}
	}

	// Load the embedded certificates.
	embeddedCerts, err := certs.GetEmbeddedCertificates()
	if err != nil {
		return nil, err
	}

	// Get the root CA.
	var rootCABuf bytes.Buffer

	err = pem.Encode(&rootCABuf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: embeddedCerts.RootCACertificate.Raw,
	})
	if err != nil {
		return nil, err
	}

	// Write the root CA to a temporary file.
	rootCAFile, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}

	defer func() { _ = os.Remove(rootCAFile.Name()) }()

	_, err = rootCAFile.Write(rootCABuf.Bytes())
	if err != nil {
		return nil, err
	}

	err = rootCAFile.Close()
	if err != nil {
		return nil, err
	}

	// Verify the signed message.
	verified := bytes.NewBuffer(nil)

	err = subprocess.RunCommandWithFds(ctx, bytes.NewReader(message), verified, "openssl", "smime", "-verify", "-text", "-CAfile", rootCAFile.Name())
	if err != nil {
		// Return a nicer error if verification failed due to a missing/unverifiable CA.
		if strings.Contains(err.Error(), "Verify error: unable to get local issuer certificate") {
			return nil, errors.New("unable to verify S/MIME message due to its use of a missing or unverifiable CA")
		}

		return nil, err
	}

	// Get the now-verified message's certificate chain.
	certChain := bytes.NewBuffer(nil)

	err = subprocess.RunCommandWithFds(ctx, bytes.NewReader(message), certChain, "openssl", "smime", "-pk7out")
	if err != nil {
		return nil, err
	}

	// openssl seems to output PEM even if `-outform DER` was provided in the command
	// above. So, we need to take an extra step to decode the output before it can be
	// parsed as pkcs7.
	block, _ := pem.Decode(certChain.Bytes())
	if block == nil {
		return nil, errors.New("unable to parse PEM-encoded pkcs7 data")
	}

	pkcs, err := pkcs7.Parse(block.Bytes)
	if err != nil {
		return nil, err
	}

	foundIntermediateCA := false

	for _, cert := range pkcs.Certificates {
		if slices.ContainsFunc(expectedIntermediateCAs, func(c *x509.Certificate) bool {
			return bytes.Equal(cert.Raw, c.Raw)
		}) {
			foundIntermediateCA = true

			break
		}
	}

	if !foundIntermediateCA {
		expectedSubjects := []string{}
		for _, ca := range expectedIntermediateCAs {
			if !slices.Contains(expectedSubjects, ca.Subject.String()) {
				expectedSubjects = append(expectedSubjects, ca.Subject.String())
			}
		}

		return nil, errors.New("S/MIME message contained a valid signature, but was not signed by one of the following expected intermediate CAs: '" + strings.Join(expectedSubjects, "', '") + "'")
	}

	return verified, nil
}
