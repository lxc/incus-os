package util

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"

	"github.com/pires/go-proxyproto"
)

// FancyTLSListener is a variation of the standard tls.Listener that supports
// atomically swapping the underlying TLS configuration.
// Requests served before the swap will continue using the old configuration.
type FancyTLSListener struct {
	net.Listener

	mu           sync.RWMutex
	config       *tls.Config
	trustedProxy []net.IP
}

// NewFancyTLSListener creates a new FancyTLSListener.
func NewFancyTLSListener(inner net.Listener, cert tls.Certificate) *FancyTLSListener {
	listener := &FancyTLSListener{
		Listener: inner,
	}

	listener.Config(cert)

	return listener
}

// Accept waits for and returns the next incoming TLS connection then use the
// current TLS configuration to handle it.
func (l *FancyTLSListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	config := l.config

	if isProxy(c.RemoteAddr().String(), l.trustedProxy) {
		c = proxyproto.NewConn(c)
	}

	return tls.Server(c, config), nil
}

// Config safely swaps the underlying TLS configuration.
func (l *FancyTLSListener) Config(cert tls.Certificate) {
	config := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequestClientCert,
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.config = config
}

// Close closes the listener.
func (l *FancyTLSListener) Close() error {
	err := l.Listener.Close()
	if err != nil {
		opErr, ok := err.(*net.OpError) //nolint:errorlint
		if !ok || !errors.Is(opErr.Err, net.ErrClosed) {
			return err
		}
	}

	return nil
}

// TrustedProxy sets new the https trusted proxy configuration.
func (l *FancyTLSListener) TrustedProxy(proxies []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.trustedProxy = make([]net.IP, 0, len(proxies))
	for _, p := range proxies {
		ip := net.ParseIP(p)
		if ip == nil {
			return fmt.Errorf("HTTPS proxy %q is not a valid IP", p)
		}

		l.trustedProxy = append(l.trustedProxy, ip)
	}

	return nil
}

func isProxy(addr string, proxies []net.IP) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}

	hostIP := net.ParseIP(host)

	return slices.ContainsFunc(proxies, hostIP.Equal)
}
