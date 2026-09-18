package http

import (
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"context"
)

// ListAgents lists active assistants and their configuration readiness.
func (h *CatalogHandler) ListAgents(ctx context.Context, r gen.ListAgentsRequestObject) (gen.ListAgentsResponseObject, error) {
	page, err := h.agents.ListAgents(ctx, requestFrom(ctx).principal, pageRequest(r.Params.Limit, r.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.Agent, len(page.Items))
	for i, a := range page.Items {
		items[i] = agentDTO(a)
	}
	return gen.ListAgents200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// GetAgent returns a configured assistant without executing it.
func (h *CatalogHandler) GetAgent(ctx context.Context, r gen.GetAgentRequestObject) (gen.GetAgentResponseObject, error) {
	a, err := h.agents.GetAgent(ctx, requestFrom(ctx).principal, r.AgentId)
	if err != nil {
		return nil, err
	}
	return gen.GetAgent200JSONResponse(agentDTO(a)), nil
}

// CreateAgent applies defaults and validates the selected provider.
func (h *CatalogHandler) CreateAgent(ctx context.Context, r gen.CreateAgentRequestObject) (gen.CreateAgentResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	c := inbound.AgentCreateCommand{Name: r.Body.Name, Description: stringValue(r.Body.Description), ProviderID: r.Body.ProviderId, Model: r.Body.Model, SystemPrompt: stringValue(r.Body.SystemPrompt), Temperature: temperatureValue(r.Body.Temperature), MaxOutputTokens: valuePointer(r.Body.MaxOutputTokens), ShowThinking: r.Body.ShowThinking, ContextWindowTokens: r.Body.ContextWindowTokens, MaxIterations: r.Body.MaxIterations, TimeoutSeconds: r.Body.TimeoutSeconds}
	a, err := h.agents.CreateAgent(ctx, requestFrom(ctx).principal, c)
	if err != nil {
		return nil, err
	}
	return gen.CreateAgent201JSONResponse(agentDTO(a)), nil
}

// UpdateAgent preserves nullable generation settings across partial updates.
func (h *CatalogHandler) UpdateAgent(ctx context.Context, r gen.UpdateAgentRequestObject) (gen.UpdateAgentResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	c := inbound.AgentUpdateCommand{Name: r.Body.Name, Description: r.Body.Description, ProviderID: r.Body.ProviderId, Model: r.Body.Model, SystemPrompt: r.Body.SystemPrompt, Temperature: domain.Change[float64]{Set: r.Body.Temperature.IsSpecified(), Value: temperatureValue(r.Body.Temperature)}, MaxOutputTokens: changeValue(r.Body.MaxOutputTokens), ShowThinking: r.Body.ShowThinking, ContextWindowTokens: r.Body.ContextWindowTokens, MaxIterations: r.Body.MaxIterations, TimeoutSeconds: r.Body.TimeoutSeconds}
	a, err := h.agents.UpdateAgent(ctx, requestFrom(ctx).principal, r.AgentId, c)
	if err != nil {
		return nil, err
	}
	return gen.UpdateAgent200JSONResponse(agentDTO(a)), nil
}

// DeleteAgent archives an assistant while retaining its historical identity.
func (h *CatalogHandler) DeleteAgent(ctx context.Context, r gen.DeleteAgentRequestObject) (gen.DeleteAgentResponseObject, error) {
	err := h.agents.DeleteAgent(ctx, requestFrom(ctx).principal, r.AgentId)
	return gen.DeleteAgent204Response{}, err
}
