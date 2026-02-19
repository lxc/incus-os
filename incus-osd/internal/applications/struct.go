package applications

import (
	"context"
	"crypto/tls"
	"io"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// Application represents an installed application.
type Application interface { //nolint:interfacebloat
	AddTrustedCertificate(ctx context.Context, name string, cert string) error
	Debug(ctx context.Context, data any) response.Response
	DebugStruct() any
	FactoryReset(ctx context.Context) error
	GetBackup(archive io.Writer, complete bool) error
	GetClientCertificate() (*tls.Certificate, error)
	GetDependencies() []string
	GetServerCertificate() (*tls.Certificate, error)
	Initialize(ctx context.Context) error
	IsPrimary() bool
	IsRunning(ctx context.Context) bool
	Name() string
	NeedsLateUpdateCheck() bool
	Restart(ctx context.Context, version string) error
	RestoreBackup(ctx context.Context, archive io.Reader) error
	Start(ctx context.Context, version string) error
	Stop(ctx context.Context, version string) error
	Update(ctx context.Context, version string) error
	WipeLocalData() error
}
