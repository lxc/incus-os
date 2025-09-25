package applications

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
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

// Common helper to grab error message from return json data.
func getErrorMessage(reader io.Reader) error {
	type resp struct {
		ErrorCode int    `json:"error_code"`
		Error     string `json:"error"`
	}

	r := &resp{}

	err := json.NewDecoder(reader).Decode(r)
	if err != nil {
		return err
	}

	if r.ErrorCode == 200 {
		return nil
	}

	return errors.New(r.Error)
}
