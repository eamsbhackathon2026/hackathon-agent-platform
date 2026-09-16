package fakes

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"context"
	"github.com/google/uuid"
	"sort"
	"time"
)

// CreateAgent checks the active-name uniqueness and provider reference.
func (s *CatalogStore) CreateAgent(ctx context.Context, a domain.Agent) error {
	defer s.lock(ctx)()
	if _, ok := s.providers[a.ProviderID]; !ok {
		return domain.ErrNotFound
	}
	for _, v := range s.agents {
		if v.ID == a.ID || (v.ArchivedAt == nil && v.Name == a.Name) {
			return domain.ErrConflict
		}
	}
	s.agents[a.ID] = cloneAgent(a)
	return nil
}

// GetAgent excludes archived assistants.
func (s *CatalogStore) GetAgent(ctx context.Context, id uuid.UUID) (domain.Agent, error) {
	defer s.lock(ctx)()
	a, ok := s.agents[id]
	if !ok || a.ArchivedAt != nil {
		return domain.Agent{}, domain.ErrNotFound
	}
	return cloneAgent(a), nil
}

// GetAgentForUpdate uses the enclosing transaction lock.
func (s *CatalogStore) GetAgentForUpdate(ctx context.Context, id uuid.UUID) (domain.Agent, error) {
	return s.GetAgent(ctx, id)
}

// ListAgents returns only active assistants in descending creation order.
func (s *CatalogStore) ListAgents(ctx context.Context, p domain.PageOptions) ([]domain.Agent, error) {
	defer s.lock(ctx)()
	items := []domain.Agent{}
	for _, a := range s.agents {
		if a.ArchivedAt == nil && before(a.CreatedAt, a.ID, p) {
			items = append(items, cloneAgent(a))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if p.Limit > 0 && len(items) > p.Limit {
		items = items[:p.Limit]
	}
	return items, nil
}

// UpdateAgent maintains active-name uniqueness and provider existence.
func (s *CatalogStore) UpdateAgent(ctx context.Context, a domain.Agent) error {
	defer s.lock(ctx)()
	old, ok := s.agents[a.ID]
	if !ok || old.ArchivedAt != nil {
		return domain.ErrNotFound
	}
	if _, ok := s.providers[a.ProviderID]; !ok {
		return domain.ErrNotFound
	}
	for _, v := range s.agents {
		if v.ID != a.ID && v.ArchivedAt == nil && v.Name == a.Name {
			return domain.ErrConflict
		}
	}
	s.agents[a.ID] = cloneAgent(a)
	return nil
}

// ArchiveAgent removes an assistant from active reads.
func (s *CatalogStore) ArchiveAgent(ctx context.Context, id uuid.UUID, at time.Time) error {
	defer s.lock(ctx)()
	a, ok := s.agents[id]
	if !ok || a.ArchivedAt != nil {
		return domain.ErrNotFound
	}
	a.ArchivedAt = &at
	a.UpdatedAt = at
	s.agents[id] = a
	return nil
}

// ListAgentsByProvider reports active references that prevent deletion.
func (s *CatalogStore) ListAgentsByProvider(ctx context.Context, id uuid.UUID) ([]domain.ResourceReference, error) {
	defer s.lock(ctx)()
	items := []domain.ResourceReference{}
	for _, a := range s.agents {
		if a.ArchivedAt == nil && a.ProviderID == id {
			items = append(items, domain.ResourceReference{ID: a.ID, Name: a.Name})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID.String() < items[j].ID.String() })
	return items, nil
}
