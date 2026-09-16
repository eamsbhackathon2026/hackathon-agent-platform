package outbound

import (
	"context"
	"encoding/json"

	"agent-platform/services/agent-service/internal/core/domain"
)

// HTTPToolInvoker executes one validated saved HTTP tool.
type HTTPToolInvoker interface {
	Invoke(context.Context, domain.HTTPTool, map[string]string, json.RawMessage) (domain.HTTPToolInvocation, error)
}

// MCPConnector opens a request-scoped session to one saved server.
type MCPConnector interface {
	Connect(context.Context, domain.MCPServer, map[string]string) (MCPSession, error)
}

// MCPSession isolates the MCP SDK from core services.
type MCPSession interface {
	ListTools(context.Context) ([]domain.MCPTool, error)
	CallTool(context.Context, string, json.RawMessage) (domain.ToolResult, error)
	Close() error
}
