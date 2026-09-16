package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// RunListOptions combines public filters, ownership and cursor pagination.
type RunListOptions struct {
	Limit                      int
	Before                     *domain.PageCursor
	AgentID                    *uuid.UUID
	Status                     *domain.RunStatus
	Source                     *domain.RunSource
	From, To                   *time.Time
	OwnerUserID, OwnerAPIKeyID *uuid.UUID
}

// SessionListOptions combines public filters, ownership and cursor pagination.
type SessionListOptions struct {
	Limit                      int
	Before                     *domain.PageCursor
	BeforeUpdated              *SessionUpdatedCursor
	OrderByUpdatedAt           bool
	AgentID                    *uuid.UUID
	Source                     *domain.RunSource
	OwnerUserID, OwnerAPIKeyID *uuid.UUID
}

// SessionUpdatedCursor identifies a stable position in descending activity order.
type SessionUpdatedCursor struct {
	UpdatedAt time.Time `json:"updated_at"`
	ID        uuid.UUID `json:"id"`
}

// MessageCursor preserves the ascending message sort key.
type MessageCursor struct {
	Seq int64
	ID  uuid.UUID
}

// SpanCursor preserves the ascending span sort key.
type SpanCursor struct {
	StartedAt time.Time
	ID        uuid.UUID
}

// SessionRepository persists active conversation containers.
type SessionRepository interface {
	CreateSession(context.Context, domain.Session) (bool, error)
	GetSession(context.Context, uuid.UUID) (domain.Session, error)
	GetSessionForUpdate(context.Context, uuid.UUID) (domain.Session, error)
	GetSessionByExternalKey(context.Context, uuid.UUID, string) (domain.Session, error)
	ListSessions(context.Context, SessionListOptions) ([]domain.Session, error)
	HasActiveRuns(context.Context, uuid.UUID) (bool, error)
	DeleteSession(context.Context, uuid.UUID, time.Time) error
	UpdatePromptTokenCalibration(context.Context, uuid.UUID, int, int, time.Time) error
}

// MessageRepository appends and reads strictly ordered conversation entries.
type MessageRepository interface {
	AppendMessage(context.Context, domain.Message) (domain.Message, error)
	ListRecentMessages(context.Context, uuid.UUID, int) ([]domain.Message, error)
	ListMessages(context.Context, uuid.UUID, int, *MessageCursor) ([]domain.Message, error)
	ListRunMessages(context.Context, uuid.UUID, uuid.UUID, int, *MessageCursor) ([]domain.Message, error)
	ListMessagesAfterSeq(context.Context, uuid.UUID, int64, int) ([]domain.Message, error)
}

// ContextSnapshotRepository persists durable, monotonic summary checkpoints.
type ContextSnapshotRepository interface {
	GetLatestContextSnapshot(context.Context, uuid.UUID) (domain.ContextSnapshot, error)
	CreateContextSnapshotCAS(context.Context, domain.ContextSnapshot, int64) (bool, error)
}

// RunRepository persists lifecycle state and cancellation requests.
type RunRepository interface {
	CreateRun(context.Context, domain.Run) error
	GetRun(context.Context, uuid.UUID) (domain.Run, error)
	StartQueuedRun(context.Context, uuid.UUID, time.Time) (domain.Run, error)
	ListRuns(context.Context, RunListOptions) ([]domain.Run, error)
	RequestRunCancel(context.Context, uuid.UUID, time.Time) (domain.Run, error)
	RunCancelRequested(context.Context, uuid.UUID) (bool, error)
	FinishRun(context.Context, domain.Run) (domain.Run, error)
}

// SpanRepository stores completed, public-safe trace steps.
type SpanRepository interface {
	CreateSpan(context.Context, domain.Span) error
	ListSpans(context.Context, uuid.UUID, int, *SpanCursor) ([]domain.Span, error)
}
