package inbound

import (
	"context"

	"agent-platform/services/agent-service/internal/core/domain"
)

// AsyncRunProcessor executes one leased job without retrying side effects.
type AsyncRunProcessor interface {
	ProcessJob(context.Context, string, domain.RunJob) error
}
