package postgres

import (
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"context"
	"github.com/google/uuid"
	"time"
)

var _ outbound.AgentRepository = (*Store)(nil)

// CreateAgent inserts an assistant configuration.
func (s *Store) CreateAgent(ctx context.Context, v domain.Agent) error {
	if v.ContextWindowTokens == 0 {
		v.ContextWindowTokens = domain.DefaultContextWindowTokens
	}
	if v.ContextWindowTokens < domain.MinContextWindowTokens || v.ContextWindowTokens > domain.MaxContextWindowTokens || v.MaxIterations < 1 || v.MaxIterations > 25 || v.TimeoutSeconds < 10 || v.TimeoutSeconds > 600 {
		return domain.ErrValidation
	}
	if domain.EffectiveMaxOutputTokens(v) > domain.MaxOutputTokensForContext(v.ContextWindowTokens) {
		return domain.ErrValidation
	}
	tokens, err := intValue(v.MaxOutputTokens)
	if err != nil {
		return err
	}
	return mapError(s.queries(ctx).CreateAgent(ctx, sqlcgen.CreateAgentParams{ID: dbID(v.ID), Name: v.Name, Description: v.Description, ProviderID: agentProviderID(v.ProviderID), Model: v.Model, SystemPrompt: v.SystemPrompt, Temperature: floatValue(v.Temperature), MaxOutputTokens: tokens, ContextWindowTokens: int32(v.ContextWindowTokens), MaxIterations: int32(v.MaxIterations), TimeoutSeconds: int32(v.TimeoutSeconds), CreatedBy: dbID(v.CreatedBy), ArchivedAt: optionalTime(v.ArchivedAt), CreatedAt: catalogTime(v.CreatedAt), UpdatedAt: catalogTime(v.UpdatedAt)}))
}

// GetAgent returns only an active assistant.
func (s *Store) GetAgent(ctx context.Context, id uuid.UUID) (domain.Agent, error) {
	v, err := s.queries(ctx).GetAgent(ctx, dbID(id))
	return agentModel(v), mapError(err)
}

// GetAgentForUpdate locks an active assistant for the current transaction.
func (s *Store) GetAgentForUpdate(ctx context.Context, id uuid.UUID) (domain.Agent, error) {
	v, err := s.queries(ctx).GetAgentForUpdate(ctx, dbID(id))
	return agentModel(v), mapError(err)
}

// ListAgents returns active assistants in descending creation order.
func (s *Store) ListAgents(ctx context.Context, p domain.PageOptions) ([]domain.Agent, error) {
	limit, before, id := page(p)
	rows, err := s.queries(ctx).ListAgents(ctx, sqlcgen.ListAgentsParams{Limit: limit, BeforeTime: before, BeforeID: id})
	result := make([]domain.Agent, 0, len(rows))
	for _, v := range rows {
		result = append(result, agentModel(v))
	}
	return result, mapError(err)
}

// UpdateAgent replaces editable fields of an active assistant.
func (s *Store) UpdateAgent(ctx context.Context, v domain.Agent) error {
	if v.ContextWindowTokens < domain.MinContextWindowTokens || v.ContextWindowTokens > domain.MaxContextWindowTokens || v.MaxIterations < 1 || v.MaxIterations > 25 || v.TimeoutSeconds < 10 || v.TimeoutSeconds > 600 {
		return domain.ErrValidation
	}
	if domain.EffectiveMaxOutputTokens(v) > domain.MaxOutputTokensForContext(v.ContextWindowTokens) {
		return domain.ErrValidation
	}
	tokens, err := intValue(v.MaxOutputTokens)
	if err != nil {
		return err
	}
	return affected(s.queries(ctx).UpdateAgent(ctx, sqlcgen.UpdateAgentParams{ID: dbID(v.ID), Name: v.Name, Description: v.Description, ProviderID: agentProviderID(v.ProviderID), Model: v.Model, SystemPrompt: v.SystemPrompt, Temperature: floatValue(v.Temperature), MaxOutputTokens: tokens, ContextWindowTokens: int32(v.ContextWindowTokens), MaxIterations: int32(v.MaxIterations), TimeoutSeconds: int32(v.TimeoutSeconds), UpdatedAt: catalogTime(v.UpdatedAt)}))
}

// ArchiveAgent hides an assistant while preserving its historical ID.
func (s *Store) ArchiveAgent(ctx context.Context, id uuid.UUID, at time.Time) error {
	return affected(s.queries(ctx).ArchiveAgent(ctx, sqlcgen.ArchiveAgentParams{ID: dbID(id), ArchivedAt: catalogTime(at)}))
}

// ListAgentsByProvider identifies active assistants preventing connection deletion.
func (s *Store) ListAgentsByProvider(ctx context.Context, id uuid.UUID) ([]domain.ResourceReference, error) {
	rows, err := s.queries(ctx).ListAgentsByProvider(ctx, dbID(id))
	result := make([]domain.ResourceReference, 0, len(rows))
	for _, v := range rows {
		result = append(result, domain.ResourceReference{ID: uuid.UUID(v.ID.Bytes), Name: v.Name})
	}
	return result, mapError(err)
}
