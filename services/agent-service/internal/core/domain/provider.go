package domain

import (
	"github.com/google/uuid"
	"time"
)

// ProviderKind selects the provider protocol, isolated behind an LLM adapter.
type ProviderKind string

// Supported provider protocols.
const (
	ProviderGemini           ProviderKind = "gemini"
	ProviderGreenNode        ProviderKind = "greennode"
	ProviderOpenAICompatible ProviderKind = "openai_compatible"
)

// ConnectionStatus records the most recent explicit connection check.
type ConnectionStatus string

// Connection checks begin unchecked and record success or failure explicitly.
const (
	ConnectionUnchecked ConnectionStatus = "unchecked"
	ConnectionOK        ConnectionStatus = "ok"
	ConnectionFailing   ConnectionStatus = "failing"
)

// ProviderFailure contains only stable codes and safe, user-facing explanations.
type ProviderFailure struct{ Code, Message string }

// Provider stores encrypted credentials; adapters must never serialize this entity directly.
type Provider struct {
	ID                   uuid.UUID
	Name                 string
	Kind                 ProviderKind
	BaseURL              *string
	DefaultModel         *string
	APIKeyCiphertext     []byte
	APIKeyHint           *string
	Status               ConnectionStatus
	LastError            *ProviderFailure
	LastCheckedAt        *time.Time
	CreatedAt, UpdatedAt time.Time
	Revision             int64
}

// ProviderConnection is a request-scoped decrypted connection, never a public DTO.
type ProviderConnection struct {
	Kind            ProviderKind
	BaseURL, APIKey string
}

// Change distinguishes omitted fields from explicit null and replacement values.
type Change[T any] struct {
	Set   bool
	Value *T
}

// ResourceReference identifies an item that prevents a destructive operation.
type ResourceReference struct {
	ID   uuid.UUID
	Name string
}
