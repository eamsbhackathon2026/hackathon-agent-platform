//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"github.com/google/uuid"
)

func TestCatalogConstraintsAndRollback(t *testing.T) {
	s, p, a := catalogFixture(t)
	ctx := context.Background()
	if err := s.CreateProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(*domain.Agent)
		want   error
	}{
		{"active requires provider", func(a *domain.Agent) { a.ProviderID = uuid.Nil }, domain.ErrValidation},
		{"provider foreign key", func(a *domain.Agent) { a.ProviderID = uuid.New() }, domain.ErrNotFound},
		{"creator foreign key", func(a *domain.Agent) { a.CreatedBy = uuid.New() }, domain.ErrNotFound},
		{"iterations", func(a *domain.Agent) { a.MaxIterations = 26 }, domain.ErrValidation},
		{"timeout", func(a *domain.Agent) { a.TimeoutSeconds = 9 }, domain.ErrValidation},
		{"temperature", func(a *domain.Agent) { n := 2.1; a.Temperature = &n }, domain.ErrValidation},
		{"tokens", func(a *domain.Agent) { n := 0; a.MaxOutputTokens = &n }, domain.ErrValidation},
		{"context capacity", func(a *domain.Agent) { a.ContextWindowTokens = domain.MinContextWindowTokens - 1 }, domain.ErrValidation},
		{"output exhausts input budget", func(a *domain.Agent) {
			a.ContextWindowTokens = domain.MinContextWindowTokens
			n := domain.MaxOutputTokensForContext(domain.MinContextWindowTokens) + 1
			a.MaxOutputTokens = &n
		}, domain.ErrValidation},
		{"token overflow", func(a *domain.Agent) { n := 1<<32 + 512; a.MaxOutputTokens = &n }, domain.ErrValidation},
		{"iteration overflow", func(a *domain.Agent) { a.MaxIterations = 1<<32 + 8 }, domain.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := a
			v.ID = uuid.New()
			tt.change(&v)
			requireCatalogError(t, s.CreateAgent(ctx, v), tt.want)
		})
	}
	rollback := errors.New("rollback")
	err := s.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.LockCatalog(ctx); err != nil {
			return err
		}
		if _, err := s.GetProviderForUpdate(ctx, p.ID); err != nil {
			return err
		}
		if err := s.CreateAgent(ctx, a); err != nil {
			return err
		}
		return rollback
	})
	requireCatalogError(t, err, rollback)
	_, err = s.GetAgent(ctx, a.ID)
	requireCatalogError(t, err, domain.ErrNotFound)
	requireCatalogError(t, s.UpdateAgent(ctx, a), domain.ErrNotFound)
	requireCatalogError(t, s.ArchiveAgent(ctx, a.ID, time.Now()), domain.ErrNotFound)
	archived := a
	archived.ProviderID = uuid.Nil
	archived.ArchivedAt = &a.CreatedAt
	if err = s.CreateAgent(ctx, archived); err != nil {
		t.Fatalf("detached archive: %v", err)
	}
}

func TestCatalogAdvisoryLock(t *testing.T) {
	s, _, _ := catalogFixture(t)
	ctx := context.Background()
	if err := s.LockCatalog(ctx); err == nil {
		t.Fatal("lock outside transaction succeeded")
	}
	err := s.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.LockCatalog(ctx); err != nil {
			return err
		}
		// A separate connection proves that the lock lives in PostgreSQL, not Go memory.
		conn, err := s.pool.Acquire(ctx)
		if err != nil {
			return err
		}
		defer conn.Release()
		var available bool
		if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(724391821)").Scan(&available); err != nil {
			return err
		}
		if available {
			t.Fatal("catalog lock did not exclude another connection")
		}
		if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(724391820)").Scan(&available); err != nil {
			return err
		}
		if !available {
			t.Fatal("catalog lock collided with identity lock")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var available bool
	if err = s.pool.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(724391821)").Scan(&available); err != nil || !available {
		t.Fatalf("lock not released: %v %v", available, err)
	}
}

func TestCatalogCursorPagination(t *testing.T) {
	s, p, a := catalogFixture(t)
	ctx := context.Background()
	for i := range 3 {
		provider := p
		provider.ID = uuid.New()
		provider.Name = provider.ID.String()
		provider.CreatedAt = p.CreatedAt.Add(time.Duration(i) * time.Second)
		if err := s.CreateProvider(ctx, provider); err != nil {
			t.Fatal(err)
		}
		agent := a
		agent.ID = uuid.New()
		agent.Name = agent.ID.String()
		agent.ProviderID = provider.ID
		agent.CreatedAt = provider.CreatedAt
		if err := s.CreateAgent(ctx, agent); err != nil {
			t.Fatal(err)
		}
	}
	providers, err := s.ListProviders(ctx, domain.PageOptions{Limit: 2})
	if err != nil || len(providers) != 2 {
		t.Fatalf("page: %v %v", providers, err)
	}
	agents, err := s.ListAgents(ctx, domain.PageOptions{Limit: 2})
	if err != nil || len(agents) != 2 {
		t.Fatalf("page: %v %v", agents, err)
	}
	nextProviders, err := s.ListProviders(ctx, domain.PageOptions{Limit: 2, Before: &domain.PageCursor{CreatedAt: providers[1].CreatedAt, ID: providers[1].ID}})
	if err != nil || len(nextProviders) != 1 || !nextProviders[0].CreatedAt.Before(providers[1].CreatedAt) {
		t.Fatalf("next page: %v %v", nextProviders, err)
	}
	nextAgents, err := s.ListAgents(ctx, domain.PageOptions{Limit: 2, Before: &domain.PageCursor{CreatedAt: agents[1].CreatedAt, ID: agents[1].ID}})
	if err != nil || len(nextAgents) != 1 || !nextAgents[0].CreatedAt.Before(agents[1].CreatedAt) {
		t.Fatalf("next page: %v %v", nextAgents, err)
	}
}
