package applications

import (
	"context"
	"crypto/tls"
	"io"
)

// Application represents an installed application.
type Application interface { //nolint:interfacebloat
	AddTrustedCertificate(ctx context.Context, name string, cert string) error
	FactoryReset(ctx context.Context) error
	GetBackup(archive io.Writer, complete bool) error
	GetCertificate() (*tls.Certificate, error)
	Initialize(ctx context.Context) error
	IsPrimary() bool
	IsRunning(ctx context.Context) bool
	RestoreBackup(archive io.Reader) error
	Start(ctx context.Context, version string) error
	Stop(ctx context.Context, version string) error
	Update(ctx context.Context, version string) error
	WipeLocalData() error
}
