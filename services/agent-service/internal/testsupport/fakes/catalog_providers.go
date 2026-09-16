package fakes

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"context"
	"github.com/google/uuid"
	"sort"
	"time"
)

// CreateProvider persists an independent encrypted entity.
func (s *CatalogStore) CreateProvider(ctx context.Context, p domain.Provider) error {
	defer s.lock(ctx)()
	for _, v := range s.providers {
		if v.Name == p.Name || v.ID == p.ID {
			return domain.ErrConflict
		}
	}
	if p.Revision == 0 {
		p.Revision = 1
	}
	s.providers[p.ID] = cloneProvider(p)
	return nil
}

// GetProvider retrieves a copied entity.
func (s *CatalogStore) GetProvider(ctx context.Context, id uuid.UUID) (domain.Provider, error) {
	defer s.lock(ctx)()
	p, ok := s.providers[id]
	if !ok {
		return p, domain.ErrNotFound
	}
	return cloneProvider(p), nil
}

// GetProviderForUpdate shares transaction serialization with writes.
func (s *CatalogStore) GetProviderForUpdate(ctx context.Context, id uuid.UUID) (domain.Provider, error) {
	return s.GetProvider(ctx, id)
}

// ListProviders applies descending cursor pagination.
func (s *CatalogStore) ListProviders(ctx context.Context, p domain.PageOptions) ([]domain.Provider, error) {
	defer s.lock(ctx)()
	items := []domain.Provider{}
	for _, v := range s.providers {
		if before(v.CreatedAt, v.ID, p) {
			items = append(items, cloneProvider(v))
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

// UpdateProvider increments the stored revision for CAS checks.
func (s *CatalogStore) UpdateProvider(ctx context.Context, p domain.Provider) (domain.Provider, error) {
	defer s.lock(ctx)()
	old, ok := s.providers[p.ID]
	if !ok {
		return p, domain.ErrNotFound
	}
	for _, v := range s.providers {
		if v.ID != p.ID && v.Name == p.Name {
			return p, domain.ErrConflict
		}
	}
	p.Revision = old.Revision + 1
	s.providers[p.ID] = cloneProvider(p)
	return cloneProvider(p), nil
}

// DeleteProvider detaches archived assistants and protects active references.
func (s *CatalogStore) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	defer s.lock(ctx)()
	if _, ok := s.providers[id]; !ok {
		return domain.ErrNotFound
	}
	for _, a := range s.agents {
		if a.ProviderID == id && a.ArchivedAt == nil {
			return domain.ErrConflict
		}
	}
	delete(s.providers, id)
	for k, a := range s.agents {
		if a.ProviderID == id {
			a.ProviderID = uuid.Nil
			s.agents[k] = a
		}
	}
	return nil
}

// RecordProviderCheck only saves checks for the unchanged configuration.
func (s *CatalogStore) RecordProviderCheck(ctx context.Context, id uuid.UUID, revision int64, status domain.ConnectionStatus, failure *domain.ProviderFailure, at time.Time) (bool, error) {
	defer s.lock(ctx)()
	p, ok := s.providers[id]
	if !ok || p.Revision != revision {
		return false, nil
	}
	p.Status = status
	p.LastError = copyPointer(failure)
	p.LastCheckedAt = &at
	p.UpdatedAt = at
	s.providers[id] = p
	return true, nil
}
