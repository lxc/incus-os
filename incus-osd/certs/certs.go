package certs

import (
	"crypto/x509"
	"embed"
	"encoding/pem"
	"errors"
	"os"
	"strings"
)

//go:embed files/*
var files embed.FS

var parsedCerts Certificates

// Certificates holds a variety of certificates and associated metadata used by the IncusOS daemon.
type Certificates struct {
	ProductionCertSubjectKeyIDs []string

	RootCACertificate       *x509.Certificate
	SecureBootCACertificate *x509.Certificate
	HotfixCACertificate     *x509.Certificate
	UpdateCACertificate     *x509.Certificate

	SecureBootCertificates SecureBootCertificates

	AuthenticationKey *x509.Certificate
}

// SecureBootCertificates holds SecureBoot-specific certificates.
type SecureBootCertificates struct {
	PK  *x509.Certificate
	KEK []*x509.Certificate
	DB  []*x509.Certificate
	DBX []*x509.Certificate
}

// GetEmbeddedCertificates returns certificates and associated metadata that have been embedded
// into the IncusOS daemon.
func GetEmbeddedCertificates() (Certificates, error) { //nolint:revive
	// If we've already parsed the certificates once, return the cached copy.
	if parsedCerts.RootCACertificate != nil {
		return parsedCerts, nil
	}

	// Get a list of the production certificates.
	contents, err := files.ReadFile("files/production-cert-subject-key-ids.txt")
	if err != nil {
		return parsedCerts, err
	}

	parsedCerts.ProductionCertSubjectKeyIDs = strings.Split(string(contents), "\n")

	// Decode the root CA cert.
	rootCAContents, err := files.ReadFile("files/root-E1.crt")
	if err != nil {
		return parsedCerts, err
	}

	pemBlock, _ := pem.Decode(rootCAContents)
	if pemBlock == nil {
		return parsedCerts, errors.New("unable to decode root CA certificate")
	}

	if pemBlock.Type != "CERTIFICATE" {
		return parsedCerts, errors.New("root CA certificate isn't PEM-encoded")
	}

	parsedCerts.RootCACertificate, err = x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return parsedCerts, err
	}

	// Decode the SecureBoot CA cert.
	secureBootCAContents, err := files.ReadFile("files/secureboot-E1.crt")
	if err != nil {
		return parsedCerts, err
	}

	pemBlock, _ = pem.Decode(secureBootCAContents)
	if pemBlock == nil {
		return parsedCerts, errors.New("unable to decode SecureBoot CA certificate")
	}

	if pemBlock.Type != "CERTIFICATE" {
		return parsedCerts, errors.New("SecureBoot CA certificate isn't PEM-encoded")
	}

	parsedCerts.SecureBootCACertificate, err = x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return parsedCerts, err
	}

	// Decode the update CA cert.
	contents, err = files.ReadFile("files/update-E1.crt")
	if err != nil {
		return parsedCerts, err
	}

	pemBlock, _ = pem.Decode(contents)
	if pemBlock == nil {
		return parsedCerts, errors.New("unable to decode update CA certificate")
	}

	if pemBlock.Type != "CERTIFICATE" {
		return parsedCerts, errors.New("update CA certificate isn't PEM-encoded")
	}

	parsedCerts.UpdateCACertificate, err = x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return parsedCerts, err
	}

	// Decode the hotfix CA cert.
	contents, err = files.ReadFile("files/hotfix-E1.crt")
	if err == nil { //nolint:gocritic
		pemBlock, _ = pem.Decode(contents)
		if pemBlock == nil {
			return parsedCerts, errors.New("unable to decode hotfix CA certificate")
		}

		if pemBlock.Type != "CERTIFICATE" {
			return parsedCerts, errors.New("hotfix CA certificate isn't PEM-encoded")
		}

		parsedCerts.HotfixCACertificate, err = x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return parsedCerts, err
		}
	} else if os.IsNotExist(err) {
		// If a hotfix-specific CA isn't available, use the update CA instead.
		parsedCerts.HotfixCACertificate = parsedCerts.UpdateCACertificate
	} else {
		return parsedCerts, err
	}

	// Prepare CA pools to validate each SecureBoot certificate we process.
	rootCAs := x509.NewCertPool()

	ok := rootCAs.AppendCertsFromPEM(rootCAContents)
	if !ok {
		return parsedCerts, errors.New("unable to append root CA certificate to root CA pool")
	}

	intermediateCAs := x509.NewCertPool()

	ok = intermediateCAs.AppendCertsFromPEM(secureBootCAContents)
	if !ok {
		return parsedCerts, errors.New("unable to append intermediate CA certificate to intermediate CA pool")
	}

	verifyOpts := x509.VerifyOptions{
		Roots:         rootCAs,
		Intermediates: intermediateCAs,
	}

	// Decode the SecureBoot PK cert.
	contents, err = files.ReadFile("files/secureboot-PK-R1.crt")
	if err != nil {
		return parsedCerts, err
	}

	pemBlock, _ = pem.Decode(contents)
	if pemBlock == nil {
		return parsedCerts, errors.New("unable to decode SecureBoot PK certificate")
	}

	if pemBlock.Type != "CERTIFICATE" {
		return parsedCerts, errors.New("SecureBoot PK certificate isn't PEM-encoded")
	}

	parsedCerts.SecureBootCertificates.PK, err = x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return parsedCerts, err
	}

	if parsedCerts.SecureBootCertificates.PK.PublicKeyAlgorithm != x509.RSA {
		return parsedCerts, errors.New("SecureBoot PK certificate must be RSA")
	}

	_, err = parsedCerts.SecureBootCertificates.PK.Verify(verifyOpts)
	if err != nil {
		return parsedCerts, err
	}

	certs, err := files.ReadDir("files")
	if err != nil {
		return parsedCerts, err
	}

	// Decode the SecureBoot KEK cert(s).
	for _, cert := range certs {
		if !strings.HasPrefix(cert.Name(), "secureboot-KEK-") || !strings.HasSuffix(cert.Name(), ".crt") {
			continue
		}

		contents, err := files.ReadFile("files/" + cert.Name())
		if err != nil {
			return parsedCerts, err
		}

		pemBlock, _ := pem.Decode(contents)
		if pemBlock == nil {
			return parsedCerts, errors.New("unable to decode SecureBoot KEK certificate '" + cert.Name() + "'")
		}

		if pemBlock.Type != "CERTIFICATE" {
			return parsedCerts, errors.New("SecureBoot KEK certificate '" + cert.Name() + "' isn't PEM-encoded")
		}

		c, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return parsedCerts, err
		}

		if c.PublicKeyAlgorithm != x509.RSA {
			return parsedCerts, errors.New("SecureBoot KEK certificate '" + cert.Name() + "' must be RSA")
		}

		_, err = c.Verify(verifyOpts)
		if err != nil {
			return parsedCerts, err
		}

		parsedCerts.SecureBootCertificates.KEK = append(parsedCerts.SecureBootCertificates.KEK, c)
	}

	// Decode the SecureBoot db cert(s).
	for _, cert := range certs {
		if !strings.HasPrefix(cert.Name(), "secureboot-DB-") || !strings.HasSuffix(cert.Name(), ".crt") {
			continue
		}

		contents, err := files.ReadFile("files/" + cert.Name())
		if err != nil {
			return parsedCerts, err
		}

		pemBlock, _ := pem.Decode(contents)
		if pemBlock == nil {
			return parsedCerts, errors.New("unable to decode SecureBoot db certificate '" + cert.Name() + "'")
		}

		if pemBlock.Type != "CERTIFICATE" {
			return parsedCerts, errors.New("SecureBoot db certificate '" + cert.Name() + "' isn't PEM-encoded")
		}

		c, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return parsedCerts, err
		}

		if c.PublicKeyAlgorithm != x509.RSA {
			return parsedCerts, errors.New("SecureBoot db certificate '" + cert.Name() + "' must be RSA")
		}

		_, err = c.Verify(verifyOpts)
		if err != nil {
			return parsedCerts, err
		}

		parsedCerts.SecureBootCertificates.DB = append(parsedCerts.SecureBootCertificates.DB, c)
	}

	// Decode the SecureBoot dbx cert(s).
	for _, cert := range certs {
		if !strings.HasPrefix(cert.Name(), "secureboot-DBX-") || !strings.HasSuffix(cert.Name(), ".crt") {
			continue
		}

		contents, err := files.ReadFile("files/" + cert.Name())
		if err != nil {
			return parsedCerts, err
		}

		pemBlock, _ := pem.Decode(contents)
		if pemBlock == nil {
			return parsedCerts, errors.New("unable to decode SecureBoot dbx certificate '" + cert.Name() + "'")
		}

		if pemBlock.Type != "CERTIFICATE" {
			return parsedCerts, errors.New("SecureBoot dbx certificate '" + cert.Name() + "' isn't PEM-encoded")
		}

		c, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return parsedCerts, err
		}

		if c.PublicKeyAlgorithm != x509.RSA {
			return parsedCerts, errors.New("SecureBoot dbx certificate '" + cert.Name() + "' must be RSA")
		}

		_, err = c.Verify(verifyOpts)
		if err != nil {
			return parsedCerts, err
		}

		parsedCerts.SecureBootCertificates.DBX = append(parsedCerts.SecureBootCertificates.DBX, c)
	}

	// Decode the authentication key, if it exists.
	contents, err = files.ReadFile("files/auth.crt")
	if err == nil {
		pemBlock, _ = pem.Decode(contents)
		if pemBlock == nil {
			return parsedCerts, errors.New("unable to decode authentication certificate")
		}

		if pemBlock.Type != "CERTIFICATE" {
			return parsedCerts, errors.New("authentication certificate isn't PEM-encoded")
		}

		parsedCerts.AuthenticationKey, err = x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return parsedCerts, err
		}
	} else if !os.IsNotExist(err) {
		return parsedCerts, err
	}

	return parsedCerts, nil
}
