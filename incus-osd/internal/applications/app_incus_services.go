package applications

import (
	"context"

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
	return []string{"incus"}
}

func (*incusCeph) Name() string {
	return "incus-ceph"
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
	return []string{"incus"}
}

func (*incusLinstor) Name() string {
	return "incus-linstor"
}

func (*incusLinstor) Struct() any {
	return &api.Application{}
}

func (*incusLinstor) UpdateConfig(_ context.Context, _ any) error {
	return nil
}
