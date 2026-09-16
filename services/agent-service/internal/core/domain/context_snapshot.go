package domain

import (
	"time"

	"github.com/google/uuid"
)

// ContextSnapshot summarizes a complete transcript prefix without mutating raw messages.
type ContextSnapshot struct {
	ID, SessionID     uuid.UUID
	SourceRunID       *uuid.UUID
	Version           int64
	CoveredThroughSeq int64
	Summary           string
	Usage             TokenUsage
	CreatedAt         time.Time
}
