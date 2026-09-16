package domain

import (
	"time"

	"github.com/google/uuid"
)

// Message is one durable conversation entry. ProviderMeta is private replay data.
type Message struct {
	ID, SessionID uuid.UUID
	RunID         *uuid.UUID
	Seq           int64
	Role, Content string
	ToolCalls     []ToolCall
	ToolCallID    *string
	ToolName      *string
	ProviderMeta  []byte
	IsError       bool
	CreatedAt     time.Time
}
