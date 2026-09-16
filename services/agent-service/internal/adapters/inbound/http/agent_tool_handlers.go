package http

import (
	"context"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
)

// GetAgentTools handles retrieval of an agent's enabled tool sources.
func (h *ToolingHandler) GetAgentTools(ctx context.Context, request gen.GetAgentToolsRequestObject) (gen.GetAgentToolsResponseObject, error) {
	bindings, err := h.bindings.GetAgentTools(ctx, requestFrom(ctx).principal, request.AgentId)
	if err != nil {
		return nil, err
	}
	return gen.GetAgentTools200JSONResponse(bindingsDTO(bindings)), nil
}

// ReplaceAgentTools handles atomic replacement of an agent's enabled tool sources.
func (h *ToolingHandler) ReplaceAgentTools(ctx context.Context, request gen.ReplaceAgentToolsRequestObject) (gen.ReplaceAgentToolsResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	bindings, err := h.bindings.ReplaceAgentTools(ctx, requestFrom(ctx).principal, request.AgentId, domain.AgentToolBindings{ToolIDs: request.Body.ToolIds, MCPServerIDs: request.Body.McpServerIds})
	if err != nil {
		return nil, err
	}
	return gen.ReplaceAgentTools200JSONResponse(bindingsDTO(bindings)), nil
}
