package applications

import (
	"context"

	"github.com/lxc/incus-os/incus-osd/api"
)

type debug struct {
	common
}

func (d *debug) Get(_ context.Context) (any, error) {
	return d.state.Applications.Debug, nil
}

func (*debug) Name() string {
	return "debug"
}

func (*debug) Struct() any {
	return &api.Application{}
}

func (*debug) UpdateConfig(_ context.Context, _ any) error {
	return nil
}
