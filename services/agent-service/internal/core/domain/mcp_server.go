package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MCPTool is a cached provider-neutral MCP declaration.
type MCPTool struct {
	Name, Description string
	InputSchema       json.RawMessage
}

// MCPServer stores one Streamable HTTP endpoint and its last synchronized tools.
type MCPServer struct {
	ID                      uuid.UUID
	Slug, DisplayName       string
	URL                     string
	AllowedTools            *[]string
	SecretHeadersCiphertext []byte
	SecretHeaderNames       []string
	Tools                   []MCPTool
	Status                  ConnectionStatus
	LastError               *ProviderFailure
	LastSyncedAt            *time.Time
	CreatedAt, UpdatedAt    time.Time
	Revision                int64
}

// AgentToolBindings replaces all saved tool sources for one active assistant.
type AgentToolBindings struct {
	ToolIDs, MCPServerIDs []uuid.UUID
}
