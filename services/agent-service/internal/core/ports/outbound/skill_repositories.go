package outbound

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// SkillRepository persists the shared skill library and assistant bindings.
type SkillRepository interface {
	LockSkills(context.Context) error
	CreateSkill(context.Context, domain.Skill) error
	GetSkill(context.Context, uuid.UUID) (domain.Skill, error)
	ListSkills(context.Context, domain.PageOptions) ([]domain.Skill, error)
	DeleteSkill(context.Context, uuid.UUID) error
	GetAgentSkillBindings(context.Context, uuid.UUID) (domain.AgentSkillBindings, error)
	ReplaceAgentSkillBindings(context.Context, uuid.UUID, domain.AgentSkillBindings) error
	ResolveAgentSkills(context.Context, uuid.UUID) ([]domain.Skill, error)
}

// SkillResolver composes immutable skill instructions for a run. The resolved tool specs
// come in because a skill names the tools it needs, and those references have to become
// the names this run offers the model before the instructions reach the prompt.
type SkillResolver interface {
	ResolveSystemPrompt(context.Context, uuid.UUID, string, []domain.ToolSpec) (string, error)
}
