package skills

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// ListSkills returns one page without loading archive data because only SKILL.md is persisted.
func (s *Service) ListSkills(ctx context.Context, principal domain.Principal, request inbound.PageRequest) (inbound.SkillPage, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return inbound.SkillPage{}, err
	}
	options, err := pageOptions(request)
	if err != nil {
		return inbound.SkillPage{}, err
	}
	items, err := s.Skills.ListSkills(ctx, options)
	if err != nil {
		return inbound.SkillPage{}, err
	}
	page := inbound.SkillPage{Items: items}
	if len(items) == options.Limit {
		last := items[len(items)-2]
		page.Items = items[:len(items)-1]
		page.NextCursor = encodeCursor(domain.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, nil
}

// ImportSkill validates and persists one immutable Markdown or ZIP package.
func (s *Service) ImportSkill(ctx context.Context, principal domain.Principal, command inbound.SkillImportCommand) (skill domain.Skill, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return skill, err
	}
	skill, err = parseUpload(command.Filename, command.Content)
	if err != nil {
		return domain.Skill{}, err
	}
	skill.ID, err = s.IDs.NewID()
	if err != nil {
		return domain.Skill{}, err
	}
	skill.CreatedBy = principal.UserID
	now := s.Clock.Now()
	skill.CreatedAt, skill.UpdatedAt = now, now
	err = s.mutate(ctx, func(ctx context.Context) error { return s.Skills.CreateSkill(ctx, skill) })
	return skill, err
}

// GetSkill returns metadata and the canonical Markdown content.
func (s *Service) GetSkill(ctx context.Context, principal domain.Principal, id uuid.UUID) (domain.Skill, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return domain.Skill{}, err
	}
	return s.Skills.GetSkill(ctx, id)
}

// DeleteSkill removes the package and lets database cascades remove bindings.
func (s *Service) DeleteSkill(ctx context.Context, principal domain.Principal, id uuid.UUID) error {
	if err := domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return err
	}
	return s.mutate(ctx, func(ctx context.Context) error {
		if _, err := s.Skills.GetSkill(ctx, id); err != nil {
			return err
		}
		return s.Skills.DeleteSkill(ctx, id)
	})
}

var _ inbound.SkillUseCase = (*Service)(nil)
