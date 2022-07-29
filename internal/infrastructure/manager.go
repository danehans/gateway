package infrastructure

import (
	"context"

	"github.com/envoyproxy/gateway/internal/ir"
)

// Manager provides the scaffolding for managing infrastructure.
type Manager interface {
	CreateInfra(ctx context.Context, infra *ir.Infra) error
}
