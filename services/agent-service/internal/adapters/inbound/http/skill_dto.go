package http

import (
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
)

func skillSummaryDTO(skill domain.Skill) gen.SkillSummary {
	return gen.SkillSummary{
		Id:             skill.ID,
		Name:           skill.Name,
		Description:    skill.Description,
		SourceType:     gen.SkillSummarySourceType(skill.SourceType),
		SourceFilename: skill.SourceFilename,
		Checksum:       skill.Checksum,
		ToolRefs:       append([]string{}, skill.ToolRefs...),
		CreatedBy:      skill.CreatedBy,
		CreatedAt:      skill.CreatedAt,
		UpdatedAt:      skill.UpdatedAt,
	}
}

func skillDTO(skill domain.Skill) gen.Skill {
	return gen.Skill{
		Id:             skill.ID,
		Name:           skill.Name,
		Description:    skill.Description,
		SourceType:     gen.SkillSourceType(skill.SourceType),
		SourceFilename: skill.SourceFilename,
		Content:        skill.Content,
		Checksum:       skill.Checksum,
		ToolRefs:       append([]string{}, skill.ToolRefs...),
		CreatedBy:      skill.CreatedBy,
		CreatedAt:      skill.CreatedAt,
		UpdatedAt:      skill.UpdatedAt,
	}
}

func skillBindingsDTO(bindings domain.AgentSkillBindings) gen.AgentSkillBindings {
	return gen.AgentSkillBindings{SkillIds: bindings.SkillIDs}
}
