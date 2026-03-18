package providers

import (
	"bytes"
	"context"
	"encoding/pem"
	"path/filepath"
	"strconv"

	"github.com/lxc/incus-os/incus-osd/certs"
	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// CommonUpdate defines functions common to all update types.
type CommonUpdate interface {
	Version() string
	IsNewerThan(otherVersion string) bool

	Download(ctx context.Context, targetPath string, progressFunc func(float64)) error
}

// ApplicationUpdate represents an application to be installed on top of IncusOS.
type ApplicationUpdate interface {
	CommonUpdate

	Name() string
}

// OSUpdate represents a full OS update.
type OSUpdate interface {
	CommonUpdate

	DownloadImage(ctx context.Context, imageType string, targetPath string, progressFunc func(float64)) (string, error)
}

// SecureBootCertUpdate represents a Secure Boot UEFI certificate update (typically a db or dbx addition).
type SecureBootCertUpdate interface {
	CommonUpdate

	GetFilename() string
}

// Provider represents an update/application provider.
type Provider interface {
	ClearCache(ctx context.Context) error

	Type() string

	InstallApplication(ctx context.Context, s *state.State, appName string) (string, error)

	GetSecureBootCertUpdate(ctx context.Context) (SecureBootCertUpdate, error)
	GetOSUpdate(ctx context.Context) (OSUpdate, error)
	GetApplicationUpdate(ctx context.Context, name string) (ApplicationUpdate, error)

	Register(ctx context.Context, isFirstBoot bool) error
	RefreshRegister(ctx context.Context) error
	Deregister(ctx context.Context) error

	load(ctx context.Context) error
}

// DatetimeComparison takes two strings of the format YYYYMMDDhhmm and returns a boolean
// indicating if a > b. If either string can't be converted to an int, false is returned.
func DatetimeComparison(a string, b string) bool {
	aInt, err := strconv.Atoi(a)
	if err != nil {
		return false
	}

	bInt, err := strconv.Atoi(b)
	if err != nil {
		return false
	}

	return aInt > bInt
}

// GetUpdateCACert returns the certificate used to verify update metadata from the configured provider.
func GetUpdateCACert() (string, error) {
	embeddedCerts, err := certs.GetEmbeddedCertificates()
	if err != nil {
		return "", err
	}

	var b bytes.Buffer

	err = pem.Encode(&b, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: embeddedCerts.RootCACertificate.Raw,
	})
	if err != nil {
		return "", err
	}

	return b.String(), nil
}

func installApplication(ctx context.Context, s *state.State, p Provider, appName string) (string, error) {
	// Fetch the application from provider.
	update, err := p.GetApplicationUpdate(ctx, appName)
	if err != nil {
		return "", err
	}

	// Download the application.
	err = update.Download(ctx, filepath.Join(systemd.LocalExtensionsPath, update.Version()), nil)
	if err != nil {
		return "", err
	}

	// Verify the application is signed with a trusted key in the kernel's keyring.
	err = systemd.VerifyExtension(ctx, filepath.Join(systemd.LocalExtensionsPath, update.Version(), appName+".raw"))
	if err != nil {
		return "", err
	}

	// Reload sysext layer.
	err = systemd.RefreshExtensions(ctx)
	if err != nil {
		return "", err
	}

	// Get the application.
	app, err := applications.Load(ctx, s, appName)
	if err != nil {
		return "", err
	}

	// Start the application.
	err = app.Start(ctx)
	if err != nil {
		return "", err
	}

	// Initialize the application.
	err = app.Initialize(ctx)
	if err != nil {
		return "", err
	}

	return update.Version(), nil
}
