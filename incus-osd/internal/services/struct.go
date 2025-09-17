package services

import (
	"context"
)

// Service represents a system service.
type Service interface {
	Get(ctx context.Context) (any, error)
	ShouldStart() bool
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Struct() any
	Supported() bool
	Update(ctx context.Context, req any) error
}

type common struct{}

func (*common) Get(_ context.Context) (any, error) {
	return nil, nil //nolint:nilnil
}

func (*common) ShouldStart() bool {
	return true
}

func (*common) Start(_ context.Context) error {
	return nil
}

func (*common) Stop(_ context.Context) error {
	return nil
}

func (*common) Struct() any {
	return nil
}

func (*common) Supported() bool {
	return true
}

func (*common) Update(_ context.Context, _ any) error {
	return nil
}
