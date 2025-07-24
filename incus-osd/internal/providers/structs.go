package providers

import (
	"context"
	"strconv"
)

// Application represents an application to be installed on top of Incus OS.
type Application interface {
	Name() string
	Version() string
	IsNewerThan(otherVersion string) bool

	Download(ctx context.Context, targetPath string, progressFunc func(float64)) error
}

// OSUpdate represents a full OS update.
type OSUpdate interface {
	Version() string
	IsNewerThan(otherVersion string) bool

	DownloadUpdate(ctx context.Context, osName string, targetPath string, progressFunc func(float64)) error
}

// SecureBootCertUpdate represents a Secure Boot UEFI certificate update (typically a db or dbx addition).
type SecureBootCertUpdate interface {
	Version() string
	IsNewerThan(otherVersion string) bool

	Download(ctx context.Context, osName string, target string) error
}

// Provider represents an update/application provider.
type Provider interface {
	ClearCache(ctx context.Context) error

	Type() string

	GetSecureBootCertUpdate(ctx context.Context, osName string) (SecureBootCertUpdate, error)
	GetOSUpdate(ctx context.Context, osName string) (OSUpdate, error)
	GetApplication(ctx context.Context, name string) (Application, error)

	Register(ctx context.Context) error
	RefreshRegister(ctx context.Context) error

	load(ctx context.Context) error
}

// datetimeComparison takes two strings of the format YYYYMMDDhhmm and returns a boolean
// indicating if a > b. If either string can't be converted to an int, false is returned.
func datetimeComparison(a string, b string) bool {
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
