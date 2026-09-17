package domain

import (
	"time"

	"github.com/google/uuid"
)

// Session groups ordered messages for one assistant and one owner.
type Session struct {
	ID, AgentID                        uuid.UUID
	Source                             RunSource
	CreatedByUserID, CreatedByAPIKeyID *uuid.UUID
	ExternalKey                        *string
	Title                              string
	LastPromptEstimatedTokens          *int
	LastPromptTokens                   *int
	CreatedAt, UpdatedAt               time.Time
}

// OwnerID returns the user or API key that may continue the session.
func (s Session) OwnerID() *uuid.UUID {
	if s.CreatedByAPIKeyID != nil {
		return s.CreatedByAPIKeyID
	}
	return s.CreatedByUserID
}

// SessionSummary condenses a conversation's runs and messages for listings, so a
// reader can spot a troubled or busy thread without opening it.
type SessionSummary struct {
	TurnCount, FailedTurnCount int
	LatestRunID                *uuid.UUID
	LatestRunStatus            *RunStatus
	// Usage sums what providers reported; a side stays nil when no run reported it.
	Usage        TokenUsage
	ProcessingMS int64
	// FirstMessage and LastMessage are short previews, not full message content.
	FirstMessage, LastMessage *string
	LastMessageRole           *string
}
