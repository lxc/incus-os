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
	AvailableVersions() []string
	ConfigureLocalStorage(ctx context.Context) error
	Debug(ctx context.Context, data any) response.Response
	DebugStruct() any
	FactoryReset(ctx context.Context) error
	GetBackup(archive io.Writer, complete bool) error
	GetClientCertificate() (*tls.Certificate, error)
	GetDependencies() []string
	GetServerCertificate() (*tls.Certificate, error)
	Initialize(ctx context.Context) error
	IsInitialized() bool
	IsInstalled() bool
	IsPrimary() bool
	IsRunning(ctx context.Context) bool
	Name() string
	NeedsLateUpdateCheck() bool
	Restart(ctx context.Context) error
	RestoreBackup(ctx context.Context, archive io.Reader) error
	SetVersions(version string, availableVersions []string)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	SwitchVersion(newVersion string) error
	Update(ctx context.Context) error
	WipeLocalData(ctx context.Context) error
	Version() string
}
