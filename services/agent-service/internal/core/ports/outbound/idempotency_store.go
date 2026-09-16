package outbound

import (
	"context"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
)

// IdempotencyStore atomically reserves caller-scoped request keys.
type IdempotencyStore interface {
	Reserve(context.Context, domain.IdempotencyRecord, time.Time) (*domain.IdempotencyRecord, error)
	DeleteExpiredIdempotency(context.Context, time.Time) error
}
