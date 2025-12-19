package util

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// GetFilesystemTrustedCerts returns a list of certificates from the specified file
// that lives under /usr/lib/incus-osd/certs/. Only certificates that are properly
// signed by the IncusOS root CAs will be returned.
func GetFilesystemTrustedCerts(filename string) ([]x509.Certificate, error) {
	// Ensure we can't be passed a complex path to read.
	filename = filepath.Base(filename)

	// Read the trusted root and intermediate Secure Boot CA certificates.
	rootCAFile, err := os.Open("/usr/lib/incus-osd/certs/root-ca.crt")
	if err != nil {
		return nil, err
	}
	defer rootCAFile.Close()

	rootCABuf, err := io.ReadAll(rootCAFile)
	if err != nil {
		return nil, err
	}

	rootCAs := x509.NewCertPool()

	ok := rootCAs.AppendCertsFromPEM(rootCABuf)
	if !ok {
		return nil, errors.New("unable to append root CA")
	}

	securebootCAFile, err := os.Open("/usr/lib/incus-osd/certs/secureboot-ca.crt")
	if err != nil {
		return nil, err
	}
	defer securebootCAFile.Close()

	securebootCABuf, err := io.ReadAll(securebootCAFile)
	if err != nil {
		return nil, err
	}

	intermediateCAs := x509.NewCertPool()

	ok = intermediateCAs.AppendCertsFromPEM(securebootCABuf)
	if !ok {
		return nil, errors.New("unable to append intermediate CA")
	}

	// Open and parse the certificate(s).
	cert, err := os.Open(filepath.Join("/usr/lib/incus-osd/certs/", filename)) //nolint:gosec
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}
	defer cert.Close()

	certBuf, err := io.ReadAll(cert)
	if err != nil {
		return nil, err
	}

	certs := []x509.Certificate{}
	verifyOpts := x509.VerifyOptions{
		Roots:         rootCAs,
		Intermediates: intermediateCAs,
	}

	// Parse each PEM-encoded block, verify that it is trusted, and if an RSA certificate add
	// it to the list of certificates.
	for pemBlock, rest := pem.Decode(certBuf); pemBlock != nil; pemBlock, rest = pem.Decode(rest) {
		if pemBlock.Type != "CERTIFICATE" {
			continue
		}

		// Parse the certificate.
		cert, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return nil, err
		}

		// Verify the certificate is trusted.
		_, err = cert.Verify(verifyOpts)
		if err != nil {
			// Rather than returning an error if validation fails, simply skip this certificate.
			continue
		}

		switch cert.PublicKeyAlgorithm { //nolint:exhaustive
		case x509.RSA:
			certs = append(certs, *cert)
		default: // Ignore other certificate types.
		}
	}

	return certs, nil
}
