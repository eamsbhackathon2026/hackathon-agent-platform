package inbound

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// RunCommand contains validated transport-independent execution inputs.
type RunCommand struct {
	AgentID        uuid.UUID
	Input          string
	SessionID      *uuid.UUID
	SessionKey     *string
	Mode           domain.RunMode
	Metadata       json.RawMessage
	WebhookURL     *string
	IdempotencyKey *string
	RequestHash    []byte
}

// RunListRequest contains public filters plus cursor pagination.
type RunListRequest struct {
	PageRequest
	AgentID   *uuid.UUID
	SessionID *uuid.UUID
	Status    *domain.RunStatus
	Source    *domain.RunSource
	From, To  *time.Time
}

// RunPage contains one descending execution page.
type RunPage struct {
	Items      []domain.Run
	NextCursor *string
}

// SpanPage contains one ascending trace-step page.
type SpanPage struct {
	Items      []domain.Span
	NextCursor *string
}

// RunUseCase executes assistants and queries their durable results.
type RunUseCase interface {
	RunSync(context.Context, domain.Principal, RunCommand) (domain.Run, error)
	EnqueueAsync(context.Context, domain.Principal, RunCommand) (domain.Run, error)
	RunStream(context.Context, domain.Principal, RunCommand, RunEventSink) (domain.Run, error)
	CancelRun(context.Context, domain.Principal, uuid.UUID) (domain.Run, error)
	GetRun(context.Context, domain.Principal, uuid.UUID) (domain.Run, error)
	ListRuns(context.Context, domain.Principal, RunListRequest) (RunPage, error)
	ListRunSpans(context.Context, domain.Principal, uuid.UUID, PageRequest) (SpanPage, error)
}
