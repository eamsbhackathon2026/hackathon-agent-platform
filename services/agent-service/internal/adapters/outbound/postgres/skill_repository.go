package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

var _ outbound.SkillRepository = (*Store)(nil)

// LockSkills serializes library and binding mutations in one transaction.
func (s *Store) LockSkills(ctx context.Context) error {
	if _, ok := ctx.Value(txKey{s}).(pgx.Tx); !ok {
		return errors.New("skill lock requires transaction")
	}
	return mapError(s.queries(ctx).LockSkills(ctx))
}

// CreateSkill persists a validated canonical skill document and, when given, the
// uploaded file. Callers run it inside a transaction so both rows land together.
func (s *Store) CreateSkill(ctx context.Context, skill domain.Skill) error {
	queries := s.queries(ctx)
	err := queries.CreateSkill(ctx, sqlcgen.CreateSkillParams{
		ID:             dbID(skill.ID),
		Name:           skill.Name,
		Description:    skill.Description,
		SourceType:     string(skill.SourceType),
		SourceFilename: skill.SourceFilename,
		Content:        skill.Content,
		Checksum:       skill.Checksum,
		ToolRefs:       append([]string{}, skill.ToolRefs...),
		CreatedBy:      dbID(skill.CreatedBy),
		CreatedAt:      catalogTime(skill.CreatedAt),
		UpdatedAt:      catalogTime(skill.UpdatedAt),
	})
	if err != nil || len(skill.SourceFile) == 0 {
		return mapError(err)
	}
	return mapError(queries.CreateSkillSourceFile(ctx, sqlcgen.CreateSkillSourceFileParams{SkillID: dbID(skill.ID), Content: skill.SourceFile}))
}

// GetSkill returns a skill by ID.
func (s *Store) GetSkill(ctx context.Context, id uuid.UUID) (domain.Skill, error) {
	value, err := s.queries(ctx).GetSkill(ctx, dbID(id))
	if err != nil {
		return domain.Skill{}, mapError(err)
	}
	skill, err := skillModel(value.Skill)
	skill.SourceFileAvailable = value.SourceFileAvailable
	return skill, err
}

// GetSkillSourceFile returns the uploaded file kept for a skill.
func (s *Store) GetSkillSourceFile(ctx context.Context, id uuid.UUID) ([]byte, error) {
	content, err := s.queries(ctx).GetSkillSourceFile(ctx, dbID(id))
	return content, mapError(err)
}

// ListSkills returns the shared skill library using keyset pagination.
func (s *Store) ListSkills(ctx context.Context, options domain.PageOptions) ([]domain.Skill, error) {
	limit, before, id := page(options)
	rows, err := s.queries(ctx).ListSkills(ctx, sqlcgen.ListSkillsParams{Limit: limit, BeforeTime: before, BeforeID: id})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.Skill, len(rows))
	for index, row := range rows {
		result[index] = domain.Skill{
			ID:             uuid.UUID(row.ID.Bytes),
			Name:           row.Name,
			Description:    row.Description,
			SourceType:     domain.SkillSourceType(row.SourceType),
			SourceFilename: row.SourceFilename,
			Checksum:       row.Checksum,
			ToolRefs:       row.ToolRefs,
			CreatedBy:      uuid.UUID(row.CreatedBy.Bytes),
			CreatedAt:      row.CreatedAt.Time,
			UpdatedAt:      row.UpdatedAt.Time,

			SourceFileAvailable: row.SourceFileAvailable,
		}
	}
	return result, nil
}

// DeleteSkill removes a skill. Database cascades remove agent bindings.
func (s *Store) DeleteSkill(ctx context.Context, id uuid.UUID) error {
	return affected(s.queries(ctx).DeleteSkill(ctx, dbID(id)))
}

// GetAgentSkillBindings returns all skills enabled for an agent.
func (s *Store) GetAgentSkillBindings(ctx context.Context, agentID uuid.UUID) (domain.AgentSkillBindings, error) {
	ids, err := s.queries(ctx).ListAgentSkillIDs(ctx, dbID(agentID))
	if err != nil {
		return domain.AgentSkillBindings{}, mapError(err)
	}
	result := domain.AgentSkillBindings{SkillIDs: make([]uuid.UUID, len(ids))}
	for index, id := range ids {
		result.SkillIDs[index] = uuid.UUID(id.Bytes)
	}
	return result, nil
}

// ReplaceAgentSkillBindings atomically replaces bindings in the active transaction.
func (s *Store) ReplaceAgentSkillBindings(ctx context.Context, agentID uuid.UUID, bindings domain.AgentSkillBindings) error {
	queries := s.queries(ctx)
	if err := queries.DeleteAgentSkills(ctx, dbID(agentID)); err != nil {
		return mapError(err)
	}
	for _, id := range bindings.SkillIDs {
		if err := queries.AddAgentSkill(ctx, sqlcgen.AddAgentSkillParams{AgentID: dbID(agentID), SkillID: dbID(id)}); err != nil {
			return mapError(err)
		}
	}
	return nil
}

// ResolveAgentSkills snapshots complete skill documents enabled for one run.
func (s *Store) ResolveAgentSkills(ctx context.Context, agentID uuid.UUID) ([]domain.Skill, error) {
	rows, err := s.queries(ctx).ResolveAgentSkills(ctx, dbID(agentID))
	if err != nil {
		return nil, mapError(err)
	}
	return skillModels(rows)
}

func skillModels(rows []sqlcgen.Skill) ([]domain.Skill, error) {
	result := make([]domain.Skill, 0, len(rows))
	for _, row := range rows {
		skill, err := skillModel(row)
		if err != nil {
			return nil, err
		}
		result = append(result, skill)
	}
	return result, nil
}

func skillModel(value sqlcgen.Skill) (domain.Skill, error) {
	skill := domain.Skill{
		ID:             uuid.UUID(value.ID.Bytes),
		Name:           value.Name,
		Description:    value.Description,
		SourceType:     domain.SkillSourceType(value.SourceType),
		SourceFilename: value.SourceFilename,
		Content:        value.Content,
		Checksum:       value.Checksum,
		ToolRefs:       value.ToolRefs,
		CreatedBy:      uuid.UUID(value.CreatedBy.Bytes),
		CreatedAt:      value.CreatedAt.Time,
		UpdatedAt:      value.UpdatedAt.Time,
	}
	if err := domain.ValidateSkill(skill); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}
