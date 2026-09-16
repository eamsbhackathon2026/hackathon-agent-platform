package outbound

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"context"
	"github.com/google/uuid"
	"time"
)

// ProviderRepository persists encrypted connections and serializes catalog mutations.
type ProviderRepository interface {
	LockCatalog(context.Context) error
	CreateProvider(context.Context, domain.Provider) error
	GetProvider(context.Context, uuid.UUID) (domain.Provider, error)
	GetProviderForUpdate(context.Context, uuid.UUID) (domain.Provider, error)
	ListProviders(context.Context, domain.PageOptions) ([]domain.Provider, error)
	UpdateProvider(context.Context, domain.Provider) (domain.Provider, error)
	DeleteProvider(context.Context, uuid.UUID) error
	// RecordProviderCheck applies a result only if the configuration revision is unchanged.
	RecordProviderCheck(context.Context, uuid.UUID, int64, domain.ConnectionStatus, *domain.ProviderFailure, time.Time) (bool, error)
}

// AgentRepository exposes active assistants; archive preserves their historical IDs.
type AgentRepository interface {
	CreateAgent(context.Context, domain.Agent) error
	GetAgent(context.Context, uuid.UUID) (domain.Agent, error)
	GetAgentForUpdate(context.Context, uuid.UUID) (domain.Agent, error)
	ListAgents(context.Context, domain.PageOptions) ([]domain.Agent, error)
	UpdateAgent(context.Context, domain.Agent) error
	ArchiveAgent(context.Context, uuid.UUID, time.Time) error
	ListAgentsByProvider(context.Context, uuid.UUID) ([]domain.ResourceReference, error)
}
