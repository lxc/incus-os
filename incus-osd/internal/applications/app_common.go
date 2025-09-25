package applications

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/lxc/incus/v6/shared/api"
)

type common struct{}

// AddTrustedCertificate adds a new trusted certificate to the application.
func (*common) AddTrustedCertificate(_ string, _ string) error {
	return errors.New("not supported")
}

// GetCertificate gets the server certificate for the application.
func (*common) GetCertificate() (*tls.Certificate, error) {
	return nil, errors.New("not supported")
}

// Initialize runs first time initialization.
func (*common) Initialize(_ context.Context) error {
	return nil
}

// Start runs startup action.
func (*common) Start(_ context.Context, _ string) error {
	return nil
}

// Stop runs shutdown action.
func (*common) Stop(_ context.Context, _ string) error {
	return nil
}

// Update triggers a partial application restart after an update.
func (*common) Update(_ context.Context, _ string) error {
	return nil
}

// IsPrimary reports if the application is a primary application.
func (*common) IsPrimary() bool {
	return false
}

// IsRunning reports if the application is currently running.
func (*common) IsRunning(_ context.Context) bool {
	return true
}

// Common helper to construct an HTTP client using the provided local Unix socket.
func unixHTTPClient(socketPath string) (*http.Client, error) {
	// Setup a Unix socket dialer
	unixDial := func(_ context.Context, _ string, _ string) (net.Conn, error) {
		raddr, err := net.ResolveUnixAddr("unix", socketPath)
		if err != nil {
			return nil, err
		}

		return net.DialUnix("unix", nil, raddr)
	}

	// Define the http transport
	transport := &http.Transport{
		DialContext:           unixDial,
		DisableKeepAlives:     true,
		ExpectContinueTimeout: time.Second * 30,
		ResponseHeaderTimeout: time.Second * 3600,
		TLSHandshakeTimeout:   time.Second * 5,
	}

	// Define the http client
	client := &http.Client{}

	client.Transport = transport

	// Setup redirect policy
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Replicate the headers
		req.Header = via[len(via)-1].Header

		return nil
	}

	return client, nil
}

// Common helper for performing REST API calls.
func doRequest(ctx context.Context, socket string, url string, method string, body []byte) ([]byte, error) {
	client, err := unixHTTPClient(socket)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	r := &api.ResponseRaw{}

	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK || r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Error)
	}

	ret, err := json.Marshal(r.Metadata)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Comment helper to compute the SHA256 fingerprint of a PEM-encoded certificate.
func getCertificateFingerprint(certificate string) (string, error) {
	certBlock, _ := pem.Decode([]byte(certificate))
	if certBlock == nil {
		return "", errors.New("cannot parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return "", fmt.Errorf("invalid certificate: %w", err)
	}

	rawFp := sha256.Sum256(cert.Raw)

	return hex.EncodeToString(rawFp[:]), nil
}
