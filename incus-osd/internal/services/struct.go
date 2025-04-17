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
	Update(ctx context.Context, req any) error
	init(ctx context.Context) error
}
