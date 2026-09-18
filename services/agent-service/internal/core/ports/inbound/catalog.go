package inbound

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"context"
	"github.com/google/uuid"
)

// ProviderCreateCommand supplies a new encrypted connection.
type ProviderCreateCommand struct {
	Name                  string
	Kind                  domain.ProviderKind
	BaseURL, DefaultModel *string
	APIKey                string
}

// ProviderUpdateCommand preserves tri-state changes for nullable connection fields.
type ProviderUpdateCommand struct {
	Name                          *string
	Kind                          *domain.ProviderKind
	BaseURL, DefaultModel, APIKey domain.Change[string]
}

// ProviderPage contains a page of encrypted entities mapped to safe HTTP DTOs.
type ProviderPage struct {
	Items      []domain.Provider
	NextCursor *string
}

// ModelPage contains sorted unique model identifiers and an opaque cursor.
type ModelPage struct {
	Items      []string
	NextCursor *string
}

// ProviderTestResult reports a connection check without exposing upstream errors.
type ProviderTestResult struct {
	OK      bool
	Failure *domain.ProviderFailure
	Message string
}

// ProviderUseCase manages connections and administrative checks.
type ProviderUseCase interface {
	ListProviders(context.Context, domain.Principal, PageRequest) (ProviderPage, error)
	CreateProvider(context.Context, domain.Principal, ProviderCreateCommand) (domain.Provider, error)
	GetProvider(context.Context, domain.Principal, uuid.UUID) (domain.Provider, error)
	UpdateProvider(context.Context, domain.Principal, uuid.UUID, ProviderUpdateCommand) (domain.Provider, error)
	DeleteProvider(context.Context, domain.Principal, uuid.UUID) error
	TestProvider(context.Context, domain.Principal, uuid.UUID) (ProviderTestResult, error)
	ListProviderModels(context.Context, domain.Principal, uuid.UUID, PageRequest) (ModelPage, error)
}

// AgentCreateCommand supplies a new assistant, with defaults applied by the service.
type AgentCreateCommand struct {
	Name, Description                    string
	ProviderID                           uuid.UUID
	Model, SystemPrompt                  string
	Temperature                          *float64
	MaxOutputTokens, ContextWindowTokens *int
	ShowThinking                         *bool
	MaxIterations, TimeoutSeconds        *int
}

// AgentUpdateCommand supports explicit clearing of optional generation limits.
type AgentUpdateCommand struct {
	Name, Description, Model, SystemPrompt *string
	ProviderID                             *uuid.UUID
	Temperature                            domain.Change[float64]
	MaxOutputTokens                        domain.Change[int]
	ShowThinking                           *bool
	ContextWindowTokens                    *int
	MaxIterations, TimeoutSeconds          *int
}

// AgentPage contains active assistants and their readiness.
type AgentPage struct {
	Items      []domain.AgentView
	NextCursor *string
}

// AgentUseCase manages saved assistants without executing their conversations.
type AgentUseCase interface {
	ListAgents(context.Context, domain.Principal, PageRequest) (AgentPage, error)
	CreateAgent(context.Context, domain.Principal, AgentCreateCommand) (domain.AgentView, error)
	GetAgent(context.Context, domain.Principal, uuid.UUID) (domain.AgentView, error)
	UpdateAgent(context.Context, domain.Principal, uuid.UUID, AgentUpdateCommand) (domain.AgentView, error)
	DeleteAgent(context.Context, domain.Principal, uuid.UUID) error
}
