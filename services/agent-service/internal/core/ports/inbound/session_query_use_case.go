package inbound

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// SessionSort selects the descending key used by session pagination.
type SessionSort string

const (
	// SessionSortCreatedAt preserves the original creation-order listing.
	SessionSortCreatedAt SessionSort = "created_at"
	// SessionSortUpdatedAt lists conversations by their latest activity.
	SessionSortUpdatedAt SessionSort = "updated_at"
)

// SessionListRequest contains conversation filters and pagination.
type SessionListRequest struct {
	PageRequest
	AgentID  *uuid.UUID
	Source   *domain.RunSource
	MineOnly bool
	Sort     SessionSort
}

// SessionPage contains one descending conversation page.
type SessionPage struct {
	Items      []domain.Session
	NextCursor *string
}

// MessagePage contains one ascending message page.
type MessagePage struct {
	Items      []domain.Message
	NextCursor *string
}

// MessageListRequest scopes one ascending message page, optionally to one run.
type MessageListRequest struct {
	PageRequest
	RunID *uuid.UUID
}

// SessionQueryUseCase exposes JWT-only conversation history operations.
type SessionQueryUseCase interface {
	ListSessions(context.Context, domain.Principal, SessionListRequest) (SessionPage, error)
	GetSession(context.Context, domain.Principal, uuid.UUID) (domain.Session, error)
	DeleteSession(context.Context, domain.Principal, uuid.UUID) error
	ListSessionMessages(context.Context, domain.Principal, uuid.UUID, MessageListRequest) (MessagePage, error)
}
