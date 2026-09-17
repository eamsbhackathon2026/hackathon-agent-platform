package http

import "agent-platform/services/agent-service/internal/core/ports/inbound"

// ToolingHandler translates HTTP tools, MCP servers and assistant bindings.
type ToolingHandler struct {
	connections inbound.APIConnectionUseCase
	tools       inbound.ToolUseCase
	servers     inbound.MCPServerUseCase
	bindings    inbound.AgentToolBindingUseCase
	transfers   inbound.ToolTransferUseCase
}

// NewToolingHandler creates the strict HTTP adapter for all tooling use cases.
func NewToolingHandler(connections inbound.APIConnectionUseCase, tools inbound.ToolUseCase, servers inbound.MCPServerUseCase, bindings inbound.AgentToolBindingUseCase, transfers inbound.ToolTransferUseCase) *ToolingHandler {
	return &ToolingHandler{connections: connections, tools: tools, servers: servers, bindings: bindings, transfers: transfers}
}
