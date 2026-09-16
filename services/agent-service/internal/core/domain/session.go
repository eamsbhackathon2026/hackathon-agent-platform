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
