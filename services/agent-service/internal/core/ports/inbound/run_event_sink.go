package inbound

import (
	"context"

	"agent-platform/services/agent-service/internal/core/domain"
)

// RunEventSink receives ordered public events from one execution.
type RunEventSink interface {
	Emit(context.Context, domain.RunEvent) error
}

// RunEventSinkFunc adapts a function to RunEventSink.
type RunEventSinkFunc func(context.Context, domain.RunEvent) error

// Emit forwards the event to the wrapped function.
func (f RunEventSinkFunc) Emit(ctx context.Context, event domain.RunEvent) error {
	return f(ctx, event)
}
