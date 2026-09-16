package outbound

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// ToolResolver builds the enabled tool set for one assistant execution.
type ToolResolver interface {
	Resolve(context.Context, uuid.UUID) (ToolSet, error)
}

// ToolSet is request-scoped and must be closed after an execution.
type ToolSet interface {
	Specs() []domain.ToolSpec
	Execute(context.Context, domain.ToolCall) domain.ToolResult
	Close() error
}
