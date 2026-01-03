package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func certificateGenerate() ([]byte, []byte, []byte, error) {
	// Generate the certificate.
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	certTemplate := x509.Certificate{
		Subject: pkix.Name{
			Organization: []string{"Linux Containers"},
			CommonName:   "Auto-generated IncusOS client certificate",
		},

		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(10 * 365 * 24 * time.Hour),

		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},

		BasicConstraintsValid: true,
		DNSNames:              []string{"unspecified"},
	}

	certDerBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, err
	}

	keyDerBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, nil, err
	}

	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDerBytes})
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDerBytes})

	// Load the cert and key.
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get the PKCS12.
	pfx, err := pkcs12.Modern2023.Encode(cert.PrivateKey, cert.Leaf, nil, "IncusOS")
	if err != nil {
		return nil, nil, nil, err
	}

	return certBytes, keyBytes, pfx, nil
}
