//go:build integration

package postgres

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

func catalogFixture(t *testing.T) (*Store, domain.Provider, domain.Agent) {
	t.Helper()
	s := NewStore(pgtest.NewPool(t))
	now := time.Now().UTC().Truncate(time.Microsecond)
	u := domain.User{ID: uuid.New(), Email: "catalog@example.test", Name: "Owner", PasswordHash: "hash", Role: domain.RoleOwner, Status: domain.UserActive, CreatedAt: now}
	if err := s.CreateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	p := domain.Provider{ID: uuid.New(), Name: "Connection", Kind: domain.ProviderGemini, Status: domain.ConnectionUnchecked, APIKeyCiphertext: []byte{0, 255, 10, 27}, CreatedAt: now, UpdatedAt: now}
	a := domain.Agent{ID: uuid.New(), Name: "Assistant", ProviderID: p.ID, Model: "model", ContextWindowTokens: domain.DefaultContextWindowTokens, MaxIterations: 8, TimeoutSeconds: 120, CreatedBy: u.ID, CreatedAt: now, UpdatedAt: now}
	return s, p, a
}
func requireCatalogError(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("got %v; want %v", err, want)
	}
}

func TestCatalogProviderRevisionAndSecrets(t *testing.T) {
	s, p, _ := catalogFixture(t)
	ctx := context.Background()
	if err := s.CreateProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	saved, err := s.GetProvider(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 || !bytes.Equal(saved.APIKeyCiphertext, p.APIKeyCiphertext) {
		t.Fatalf("bad saved provider: %+v", saved)
	}
	duplicate := p
	duplicate.ID = uuid.New()
	requireCatalogError(t, s.CreateProvider(ctx, duplicate), domain.ErrConflict)
	checked := p.CreatedAt.Add(time.Second)
	failure := &domain.ProviderFailure{Code: "provider_auth_failed", Message: "Invalid connection"}
	ok, err := s.RecordProviderCheck(ctx, p.ID, 1, domain.ConnectionFailing, failure, checked)
	if err != nil || !ok {
		t.Fatalf("check: %v %v", ok, err)
	}
	saved, err = s.GetProvider(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 || saved.LastError == nil || *saved.LastError != *failure || !saved.LastCheckedAt.Equal(checked) {
		t.Fatalf("check not preserved: %+v", saved)
	}
	var code string
	if err = s.pool.QueryRow(ctx, "SELECT last_error->>'code' FROM llm_providers WHERE id=$1", p.ID).Scan(&code); err != nil || code != failure.Code {
		t.Fatalf("JSON: %q %v", code, err)
	}
	saved.Name = "Updated"
	saved.APIKeyCiphertext = nil
	saved.APIKeyHint = nil
	saved.Status = domain.ConnectionUnchecked
	saved.LastError = nil
	saved.LastCheckedAt = nil
	saved.UpdatedAt = checked.Add(time.Second)
	saved, err = s.UpdateProvider(ctx, saved)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 2 || saved.APIKeyCiphertext != nil || saved.APIKeyHint != nil {
		t.Fatalf("update: %+v", saved)
	}
	ok, err = s.RecordProviderCheck(ctx, p.ID, 1, domain.ConnectionFailing, failure, checked.Add(3*time.Second))
	if err != nil || ok {
		t.Fatalf("stale result applied: %v %v", ok, err)
	}
	saved, err = s.GetProvider(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != domain.ConnectionUnchecked || saved.LastError != nil || saved.LastCheckedAt != nil {
		t.Fatalf("stale state: %+v", saved)
	}
	list, err := s.ListProviders(ctx, domain.PageOptions{Limit: 1})
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if err = s.DeleteProvider(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	ok, err = s.RecordProviderCheck(ctx, p.ID, 2, domain.ConnectionOK, nil, checked)
	if err != nil || ok {
		t.Fatalf("missing check: %v %v", ok, err)
	}
	_, err = s.GetProvider(ctx, p.ID)
	requireCatalogError(t, err, domain.ErrNotFound)
	_, err = s.GetProviderForUpdate(ctx, p.ID)
	requireCatalogError(t, err, domain.ErrNotFound)
	_, err = s.UpdateProvider(ctx, saved)
	requireCatalogError(t, err, domain.ErrNotFound)
	requireCatalogError(t, s.DeleteProvider(ctx, p.ID), domain.ErrNotFound)
}

func TestCatalogArchivePreservesHistory(t *testing.T) {
	s, p, a := catalogFixture(t)
	ctx := context.Background()
	if err := s.CreateProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	temperature := 0.75
	tokens := 512
	a.Temperature = &temperature
	a.MaxOutputTokens = &tokens
	a.ShowThinking = true
	if err := s.CreateAgent(ctx, a); err != nil {
		t.Fatal(err)
	}
	saved, err := s.GetAgent(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Temperature == nil || *saved.Temperature != temperature || saved.MaxOutputTokens == nil || *saved.MaxOutputTokens != tokens || saved.ContextWindowTokens != domain.DefaultContextWindowTokens {
		t.Fatalf("nullable settings: %+v", saved)
	}
	// A column added later slips out of INSERT/UPDATE easily: only a read-back tells.
	if !saved.ShowThinking {
		t.Fatalf("the thinking flag was not stored: %+v", saved)
	}
	saved.ShowThinking = false
	if err = s.UpdateAgent(ctx, saved); err != nil {
		t.Fatal(err)
	}
	if again, agErr := s.GetAgent(ctx, a.ID); agErr != nil || again.ShowThinking {
		t.Fatalf("turning the flag off did not persist: %+v err=%v", again, agErr)
	}
	saved.ShowThinking = true
	if err = s.UpdateAgent(ctx, saved); err != nil {
		t.Fatal(err)
	}
	duplicate := a
	duplicate.ID = uuid.New()
	requireCatalogError(t, s.CreateAgent(ctx, duplicate), domain.ErrConflict)
	if err = s.DeleteProvider(ctx, p.ID); err == nil {
		t.Fatal("deleted active connection")
	}
	saved.Description = "Changed"
	saved.Temperature = nil
	saved.MaxOutputTokens = nil
	saved.UpdatedAt = a.CreatedAt.Add(time.Second)
	if err = s.UpdateAgent(ctx, saved); err != nil {
		t.Fatal(err)
	}
	saved, err = s.GetAgentForUpdate(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Description != "Changed" || saved.Temperature != nil || saved.MaxOutputTokens != nil {
		t.Fatalf("update: %+v", saved)
	}
	refs, err := s.ListAgentsByProvider(ctx, p.ID)
	if err != nil || len(refs) != 1 || refs[0].ID != a.ID {
		t.Fatalf("references: %v %v", refs, err)
	}
	if err = s.ArchiveAgent(ctx, a.ID, a.CreatedAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetAgent(ctx, a.ID)
	requireCatalogError(t, err, domain.ErrNotFound)
	_, err = s.GetAgentForUpdate(ctx, a.ID)
	requireCatalogError(t, err, domain.ErrNotFound)
	requireCatalogError(t, s.UpdateAgent(ctx, saved), domain.ErrNotFound)
	requireCatalogError(t, s.ArchiveAgent(ctx, a.ID, time.Now()), domain.ErrNotFound)
	list, err := s.ListAgents(ctx, domain.PageOptions{})
	if err != nil || len(list) != 0 {
		t.Fatalf("archive list: %v %v", list, err)
	}
	refs, err = s.ListAgentsByProvider(ctx, p.ID)
	if err != nil || len(refs) != 0 {
		t.Fatalf("archive references: %v %v", refs, err)
	}
	if err = s.DeleteProvider(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	var detached bool
	if err = s.pool.QueryRow(ctx, "SELECT provider_id IS NULL AND archived_at IS NOT NULL FROM agents WHERE id=$1", a.ID).Scan(&detached); err != nil || !detached {
		t.Fatalf("history: %v %v", detached, err)
	}
	p.ID = uuid.New()
	if err = s.CreateProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	duplicate.ProviderID = p.ID
	if err = s.CreateAgent(ctx, duplicate); err != nil {
		t.Fatal(err)
	}
}
