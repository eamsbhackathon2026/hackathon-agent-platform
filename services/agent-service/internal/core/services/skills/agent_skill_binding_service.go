package skills

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// GetAgentSkills returns the skills explicitly enabled for one assistant.
func (s *Service) GetAgentSkills(ctx context.Context, principal domain.Principal, agentID uuid.UUID) (domain.AgentSkillBindings, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return domain.AgentSkillBindings{}, err
	}
	if _, err := s.Agents.GetAgent(ctx, agentID); err != nil {
		return domain.AgentSkillBindings{}, err
	}
	return s.Skills.GetAgentSkillBindings(ctx, agentID)
}

// ReplaceAgentSkills validates the full instruction budget before atomically replacing bindings.
func (s *Service) ReplaceAgentSkills(ctx context.Context, principal domain.Principal, agentID uuid.UUID, bindings domain.AgentSkillBindings) (result domain.AgentSkillBindings, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return result, err
	}
	if len(bindings.SkillIDs) > domain.MaxAgentSkills || duplicateIDs(bindings.SkillIDs) {
		return result, domain.Invalid("skill_ids", "Chọn tối đa 20 skill và không lặp ID.")
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		if _, getErr := s.Agents.GetAgentForUpdate(ctx, agentID); getErr != nil {
			return getErr
		}
		total := 0
		for _, id := range bindings.SkillIDs {
			skill, getErr := s.Skills.GetSkill(ctx, id)
			if getErr != nil {
				return getErr
			}
			body, bodyErr := instructionBody(skill.Content)
			if bodyErr != nil {
				return bodyErr
			}
			total += len(body)
			if total > domain.MaxAgentSkillBytes {
				return domain.Invalid("skill_ids", "Tổng nội dung skill của trợ lý không được vượt quá 100 KiB.")
			}
		}
		if replaceErr := s.Skills.ReplaceAgentSkillBindings(ctx, agentID, bindings); replaceErr != nil {
			return replaceErr
		}
		result, err = s.Skills.GetAgentSkillBindings(ctx, agentID)
		return err
	})
	return result, err
}

func duplicateIDs(ids []uuid.UUID) bool {
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			return true
		}
		seen[id] = true
	}
	return false
}

var _ inbound.AgentSkillBindingUseCase = (*Service)(nil)
