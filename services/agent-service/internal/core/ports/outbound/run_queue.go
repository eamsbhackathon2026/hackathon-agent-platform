package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// RunQueue persists and leases asynchronous jobs.
type RunQueue interface {
	Enqueue(context.Context, domain.RunJob) error
	Claim(context.Context, string, int, time.Duration) ([]domain.RunJob, error)
	Extend(context.Context, uuid.UUID, string, time.Duration) error
	Complete(context.Context, uuid.UUID, string) error
}
