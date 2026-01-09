package providers

import (
	"context"
	"strconv"
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
	GetSigningCACert() (string, error)

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
