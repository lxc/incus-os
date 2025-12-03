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
	GetClientCertificate() (*tls.Certificate, error)
	GetServerCertificate() (*tls.Certificate, error)
	GetDependencies() []string
	Initialize(ctx context.Context) error
	IsPrimary() bool
	IsRunning(ctx context.Context) bool
	NeedsLateUpdateCheck() bool
	RestoreBackup(ctx context.Context, archive io.Reader) error
	Restart(ctx context.Context, version string) error
	Start(ctx context.Context, version string) error
	Stop(ctx context.Context, version string) error
	Update(ctx context.Context, version string) error
	WipeLocalData() error
}
