package inbound

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// APIConnectionCreateCommand contains plaintext secret headers only at the write boundary.
type APIConnectionCreateCommand struct {
	Slug, DisplayName, BaseURL   string
	PublicHeaders, SecretHeaders map[string]string
}

// APIConnectionUpdateCommand preserves omitted versus explicit-null secret headers.
type APIConnectionUpdateCommand struct {
	Slug, DisplayName, BaseURL *string
	PublicHeaders              *map[string]string
	SecretHeaders              domain.Change[map[string]string]
}

// APIConnectionPage is one cursor-paginated page of reusable API connections.
type APIConnectionPage struct {
	Items      []domain.APIConnection
	NextCursor *string
}

// APIConnectionUseCase manages reusable HTTP endpoint and credential configuration.
type APIConnectionUseCase interface {
	ListAPIConnections(context.Context, domain.Principal, PageRequest) (APIConnectionPage, error)
	CreateAPIConnection(context.Context, domain.Principal, APIConnectionCreateCommand) (domain.APIConnection, error)
	GetAPIConnection(context.Context, domain.Principal, uuid.UUID) (domain.APIConnection, error)
	UpdateAPIConnection(context.Context, domain.Principal, uuid.UUID, APIConnectionUpdateCommand) (domain.APIConnection, error)
	DeleteAPIConnection(context.Context, domain.Principal, uuid.UUID) error
}

// HTTPToolCreateCommand contains plaintext secret headers only at the write boundary.
type HTTPToolCreateCommand struct {
	ConnectionID                                *uuid.UUID
	Slug, DisplayName, Description, URLTemplate string
	Method                                      domain.HTTPToolMethod
	Params                                      []domain.ToolParam
	PublicHeaders, SecretHeaders                map[string]string
	TimeoutSeconds                              int
}

// HTTPToolUpdateCommand preserves omitted versus explicit-null secret headers.
type HTTPToolUpdateCommand struct {
	ConnectionID                                domain.Change[uuid.UUID]
	Slug, DisplayName, Description, URLTemplate *string
	Method                                      *domain.HTTPToolMethod
	Params                                      *[]domain.ToolParam
	PublicHeaders                               *map[string]string
	SecretHeaders                               domain.Change[map[string]string]
	TimeoutSeconds                              *int
}

// HTTPToolPage is one cursor-paginated page of saved HTTP tools.
type HTTPToolPage struct {
	Items      []domain.HTTPTool
	NextCursor *string
}

// ToolTestResult contains the safe observable result of a manual HTTP tool test.
type ToolTestResult struct {
	OK         bool
	StatusCode *int
	Body       string
	Truncated  bool
	Duration   time.Duration
	Failure    *domain.ProviderFailure
}

// ToolUseCase manages saved HTTP tools and their manual test execution.
type ToolUseCase interface {
	ListTools(context.Context, domain.Principal, PageRequest) (HTTPToolPage, error)
	CreateTool(context.Context, domain.Principal, HTTPToolCreateCommand) (domain.HTTPTool, error)
	GetTool(context.Context, domain.Principal, uuid.UUID) (domain.HTTPTool, error)
	UpdateTool(context.Context, domain.Principal, uuid.UUID, HTTPToolUpdateCommand) (domain.HTTPTool, error)
	DeleteTool(context.Context, domain.Principal, uuid.UUID) error
	TestTool(context.Context, domain.Principal, uuid.UUID, json.RawMessage) (ToolTestResult, error)
}

// MCPServerCreateCommand contains a new MCP endpoint and write-only secret headers.
type MCPServerCreateCommand struct {
	Slug, DisplayName, URL string
	AllowedTools           *[]string
	SecretHeaders          map[string]string
}

// MCPServerUpdateCommand preserves omitted versus explicit-null mutable fields.
type MCPServerUpdateCommand struct {
	Slug, DisplayName, URL *string
	AllowedTools           domain.Change[[]string]
	SecretHeaders          domain.Change[map[string]string]
}

// MCPServerPage is one cursor-paginated page of saved MCP servers.
type MCPServerPage struct {
	Items      []domain.MCPServer
	NextCursor *string
}

// MCPServerUseCase manages MCP endpoints and their cached tool catalogs.
type MCPServerUseCase interface {
	ListMCPServers(context.Context, domain.Principal, PageRequest) (MCPServerPage, error)
	CreateMCPServer(context.Context, domain.Principal, MCPServerCreateCommand) (domain.MCPServer, error)
	GetMCPServer(context.Context, domain.Principal, uuid.UUID) (domain.MCPServer, error)
	UpdateMCPServer(context.Context, domain.Principal, uuid.UUID, MCPServerUpdateCommand) (domain.MCPServer, error)
	DeleteMCPServer(context.Context, domain.Principal, uuid.UUID) error
	RefreshMCPServer(context.Context, domain.Principal, uuid.UUID) (domain.MCPServer, error)
}

// AgentToolBindingUseCase reads and atomically replaces an agent's enabled tools.
type AgentToolBindingUseCase interface {
	GetAgentTools(context.Context, domain.Principal, uuid.UUID) (domain.AgentToolBindings, error)
	ReplaceAgentTools(context.Context, domain.Principal, uuid.UUID, domain.AgentToolBindings) (domain.AgentToolBindings, error)
}
