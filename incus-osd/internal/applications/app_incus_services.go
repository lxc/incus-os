package applications

import (
	"context"
	"errors"
	"strings"

	"github.com/lxc/incus/v7/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
)

type incusCeph struct {
	common
}

func (i *incusCeph) Get(_ context.Context) (any, error) {
	return i.state.Applications.IncusCeph, nil
}

// GetDependencies returns a list of other applications this application depends on.
func (*incusCeph) GetDependencies() []string {
	return []string{incusVersionStable + " OR " + incusVersionLTS70}
}

// IsInstalled reports whether the application has been installed.
func (i *incusCeph) IsInstalled() bool {
	if i.appState.Version == "" {
		return false
	}

	return sysextImageExists(i.Name(), i.appState.Version)
}

func (*incusCeph) Name() string {
	return "incus-ceph"
}

// SetFriendlyVersion records the friendly version.
func (i *incusCeph) SetFriendlyVersion(ctx context.Context) error {
	output, err := subprocess.RunCommandContext(ctx, "ceph", "--version")
	if err != nil {
		return err
	}

	if !strings.HasPrefix(output, "ceph version ") {
		return errors.New("unable to determine ceph version")
	}

	i.appState.FriendlyVersion = strings.Split(output, " ")[2] + " [" + i.appState.Version + "]"

	return nil
}

func (*incusCeph) Struct() any {
	return &api.Application{}
}

func (*incusCeph) UpdateConfig(_ context.Context, _ any) error {
	return nil
}

type incusLinstor struct {
	common
}

func (i *incusLinstor) Get(_ context.Context) (any, error) {
	return i.state.Applications.IncusLinstor, nil
}

// GetDependencies returns a list of other applications this application depends on.
func (*incusLinstor) GetDependencies() []string {
	return []string{incusVersionStable + " OR " + incusVersionLTS70}
}

// IsInstalled reports whether the application has been installed.
func (i *incusLinstor) IsInstalled() bool {
	if i.appState.Version == "" {
		return false
	}

	return sysextImageExists(i.Name(), i.appState.Version)
}

func (*incusLinstor) Name() string {
	return "incus-linstor"
}

// SetFriendlyVersion records the friendly version.
func (i *incusLinstor) SetFriendlyVersion(ctx context.Context) error {
	output, err := subprocess.RunCommandContext(ctx, "/usr/share/linstor-server/bin/Satellite", "--version")
	if err != nil {
		return err
	}

	s := strings.Split(output, "\n")
	if !strings.HasPrefix(s[len(s)-2], "LINSTOR Satellite ") {
		return errors.New("unable to determine Linstor version")
	}

	i.appState.FriendlyVersion = strings.Split(s[len(s)-2], " ")[2] + " [" + i.appState.Version + "]"

	return nil
}

func (*incusLinstor) Struct() any {
	return &api.Application{}
}

func (*incusLinstor) UpdateConfig(_ context.Context, _ any) error {
	return nil
}
