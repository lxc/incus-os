package services

import (
	"context"
	"errors"
)

// Service represents a system service.
type Service interface {
	Get(ctx context.Context) (any, error)
	Reset(ctx context.Context) error
	ShouldStart() bool
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Struct() any
	Supported() bool
	Update(ctx context.Context, req any) error
}

type common struct{}

func (*common) Reset(_ context.Context) error {
	return errors.New("reset isn't supported by this service")
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

func (*common) Supported() bool {
	return true
}
