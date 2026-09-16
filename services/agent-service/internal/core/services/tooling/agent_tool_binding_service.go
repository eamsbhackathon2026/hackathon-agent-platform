package tooling

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// GetAgentTools returns the HTTP and MCP sources enabled for one agent.
func (s *Service) GetAgentTools(ctx context.Context, principal domain.Principal, agentID uuid.UUID) (domain.AgentToolBindings, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return domain.AgentToolBindings{}, err
	}
	if _, err := s.Agents.GetAgent(ctx, agentID); err != nil {
		return domain.AgentToolBindings{}, err
	}
	return s.Bindings.GetAgentToolBindings(ctx, agentID)
}

// ReplaceAgentTools atomically replaces every tool source enabled for one agent.
func (s *Service) ReplaceAgentTools(ctx context.Context, principal domain.Principal, agentID uuid.UUID, bindings domain.AgentToolBindings) (result domain.AgentToolBindings, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	if duplicateUUID(bindings.ToolIDs) || duplicateUUID(bindings.MCPServerIDs) {
		return result, domain.Invalid("tool_ids", "Danh sách liên kết không được chứa ID trùng.")
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		if _, getErr := s.Agents.GetAgentForUpdate(ctx, agentID); getErr != nil {
			return getErr
		}
		for _, id := range bindings.ToolIDs {
			if _, getErr := s.Tools.GetTool(ctx, id); getErr != nil {
				return getErr
			}
		}
		for _, id := range bindings.MCPServerIDs {
			if _, getErr := s.MCPServers.GetMCPServer(ctx, id); getErr != nil {
				return getErr
			}
		}
		return s.Bindings.ReplaceAgentToolBindings(ctx, agentID, bindings)
	})
	if err != nil {
		return result, err
	}
	return s.Bindings.GetAgentToolBindings(ctx, agentID)
}

func duplicateUUID(values []uuid.UUID) bool {
	seen := make(map[uuid.UUID]bool, len(values))
	for _, value := range values {
		if value == uuid.Nil || seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

var _ inbound.AgentToolBindingUseCase = (*Service)(nil)
