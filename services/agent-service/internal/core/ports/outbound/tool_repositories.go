package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// ToolRepository persists HTTP tool configuration.
type ToolRepository interface {
	LockTooling(context.Context) error
	CreateTool(context.Context, domain.HTTPTool) error
	GetTool(context.Context, uuid.UUID) (domain.HTTPTool, error)
	GetToolForUpdate(context.Context, uuid.UUID) (domain.HTTPTool, error)
	ListTools(context.Context, domain.PageOptions) ([]domain.HTTPTool, error)
	ListAllTools(context.Context) ([]domain.HTTPTool, error)
	UpdateTool(context.Context, domain.HTTPTool) error
	DeleteTool(context.Context, uuid.UUID) error
}

// APIConnectionRepository persists reusable base URLs and encrypted headers.
type APIConnectionRepository interface {
	CreateAPIConnection(context.Context, domain.APIConnection) error
	GetAPIConnection(context.Context, uuid.UUID) (domain.APIConnection, error)
	GetAPIConnectionForUpdate(context.Context, uuid.UUID) (domain.APIConnection, error)
	ListAPIConnections(context.Context, domain.PageOptions) ([]domain.APIConnection, error)
	ListAllAPIConnections(context.Context) ([]domain.APIConnection, error)
	UpdateAPIConnection(context.Context, domain.APIConnection) error
	DeleteAPIConnection(context.Context, uuid.UUID) error
	CountToolsByAPIConnection(context.Context, uuid.UUID) (int64, error)
}

// MCPServerRepository persists Streamable HTTP endpoints and synchronized caches.
type MCPServerRepository interface {
	CreateMCPServer(context.Context, domain.MCPServer) error
	GetMCPServer(context.Context, uuid.UUID) (domain.MCPServer, error)
	GetMCPServerForUpdate(context.Context, uuid.UUID) (domain.MCPServer, error)
	ListMCPServers(context.Context, domain.PageOptions) ([]domain.MCPServer, error)
	UpdateMCPServer(context.Context, domain.MCPServer) (domain.MCPServer, error)
	DeleteMCPServer(context.Context, uuid.UUID) error
	RecordMCPServerSync(context.Context, uuid.UUID, int64, []domain.MCPTool, domain.ConnectionStatus, *domain.ProviderFailure, time.Time) (bool, error)
}

// AgentToolCatalog is the coherent tool and connection snapshot for one run.
type AgentToolCatalog struct {
	HTTPTools      []domain.HTTPTool
	APIConnections []domain.APIConnection
	MCPServers     []domain.MCPServer
}

// AgentToolBindingRepository atomically replaces and resolves assistant bindings.
type AgentToolBindingRepository interface {
	GetAgentToolBindings(context.Context, uuid.UUID) (domain.AgentToolBindings, error)
	ReplaceAgentToolBindings(context.Context, uuid.UUID, domain.AgentToolBindings) error
	ResolveAgentTools(context.Context, uuid.UUID) (AgentToolCatalog, error)
}
