package inbound

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// SkillImportCommand carries one bounded upload into the application service.
type SkillImportCommand struct {
	Filename string
	Content  []byte
}

// SkillPage is one cursor-paginated page of shared skills.
type SkillPage struct {
	Items      []domain.Skill
	NextCursor *string
}

// SkillUseCase manages immutable uploaded skill instructions.
type SkillUseCase interface {
	ListSkills(context.Context, domain.Principal, PageRequest) (SkillPage, error)
	ImportSkill(context.Context, domain.Principal, SkillImportCommand) (domain.Skill, error)
	GetSkill(context.Context, domain.Principal, uuid.UUID) (domain.Skill, error)
	DownloadSkill(context.Context, domain.Principal, uuid.UUID) (domain.SkillFile, error)
	DeleteSkill(context.Context, domain.Principal, uuid.UUID) error
}

// AgentSkillBindingUseCase reads and atomically replaces an assistant's skills.
type AgentSkillBindingUseCase interface {
	GetAgentSkills(context.Context, domain.Principal, uuid.UUID) (domain.AgentSkillBindings, error)
	ReplaceAgentSkills(context.Context, domain.Principal, uuid.UUID, domain.AgentSkillBindings) (domain.AgentSkillBindings, error)
}
