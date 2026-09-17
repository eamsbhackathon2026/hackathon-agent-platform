//go:build integration

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestSkillRepositoryRoundTripBindingsAndCascade(t *testing.T) {
	store, provider, agent := catalogFixture(t)
	ctx := context.Background()
	if err := store.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatal(err)
	}
	skill := domain.Skill{ID: uuid.New(), Name: "Clear writing", Description: "Use plain language.", SourceType: domain.SkillSourceMarkdown, SourceFilename: "writing.md", Content: "# Clear writing\n\nRead $get_insights then $finance.get_portfolio.", Checksum: "0000000000000000000000000000000000000000000000000000000000000000", ToolRefs: []string{"finance.get_portfolio", "get_insights"}, CreatedBy: agent.CreatedBy, CreatedAt: time.Now().UTC().Truncate(time.Microsecond), UpdatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := store.WithinTx(ctx, func(ctx context.Context) error {
		if err := store.LockSkills(ctx); err != nil {
			return err
		}
		return store.CreateSkill(ctx, skill)
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetSkill(ctx, skill.ID)
	if err != nil || got.Name != skill.Name || got.Content != skill.Content {
		t.Fatalf("skill=%+v err=%v", got, err)
	}
	if strings.Join(got.ToolRefs, ",") != "finance.get_portfolio,get_insights" {
		t.Fatalf("tool refs from GetSkill=%v", got.ToolRefs)
	}
	items, err := store.ListSkills(ctx, domain.PageOptions{Limit: 2})
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if strings.Join(items[0].ToolRefs, ",") != "finance.get_portfolio,get_insights" {
		t.Fatalf("tool refs from ListSkills=%v", items[0].ToolRefs)
	}
	bindings := domain.AgentSkillBindings{SkillIDs: []uuid.UUID{skill.ID}}
	if err = store.WithinTx(ctx, func(ctx context.Context) error {
		if err := store.LockSkills(ctx); err != nil {
			return err
		}
		return store.ReplaceAgentSkillBindings(ctx, agent.ID, bindings)
	}); err != nil {
		t.Fatal(err)
	}
	resolved, err := store.ResolveAgentSkills(ctx, agent.ID)
	if err != nil || len(resolved) != 1 || resolved[0].ID != skill.ID {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	err = store.WithinTx(ctx, func(ctx context.Context) error {
		if err := store.LockSkills(ctx); err != nil {
			return err
		}
		return store.ReplaceAgentSkillBindings(ctx, agent.ID, domain.AgentSkillBindings{SkillIDs: []uuid.UUID{uuid.New()}})
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("invalid binding err=%v", err)
	}
	saved, err := store.GetAgentSkillBindings(ctx, agent.ID)
	if err != nil || len(saved.SkillIDs) != 1 || saved.SkillIDs[0] != skill.ID {
		t.Fatalf("rollback=%+v err=%v", saved, err)
	}
	if err = store.DeleteSkill(ctx, skill.ID); err != nil {
		t.Fatal(err)
	}
	saved, err = store.GetAgentSkillBindings(ctx, agent.ID)
	if err != nil || len(saved.SkillIDs) != 0 {
		t.Fatalf("cascade=%+v err=%v", saved, err)
	}
	if err = store.LockSkills(ctx); err == nil {
		t.Fatal("lock outside transaction succeeded")
	}
}

func TestSkillSourceFileIsKeptApartAndRemovedWithTheSkill(t *testing.T) {
	store, provider, agent := catalogFixture(t)
	ctx := context.Background()
	if err := store.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	checksum := strings.Repeat("0", 64)
	withFile := domain.Skill{ID: uuid.New(), Name: "Packaged", SourceType: domain.SkillSourceZIP, SourceFilename: "pack.zip", SourceFile: []byte("PK\x03\x04zip"), Content: "# Packaged", Checksum: checksum, ToolRefs: []string{}, CreatedBy: agent.CreatedBy, CreatedAt: now, UpdatedAt: now}
	legacy := domain.Skill{ID: uuid.New(), Name: "Legacy", SourceType: domain.SkillSourceZIP, SourceFilename: "legacy.zip", Content: "# Legacy", Checksum: checksum, ToolRefs: []string{}, CreatedBy: agent.CreatedBy, CreatedAt: now.Add(-time.Second), UpdatedAt: now}
	for _, skill := range []domain.Skill{withFile, legacy} {
		if err := store.WithinTx(ctx, func(ctx context.Context) error { return store.CreateSkill(ctx, skill) }); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.GetSkill(ctx, withFile.ID)
	if err != nil || !got.SourceFileAvailable || got.SourceFile != nil {
		t.Fatalf("with file=%+v err=%v", got, err)
	}
	content, err := store.GetSkillSourceFile(ctx, withFile.ID)
	if err != nil || string(content) != string(withFile.SourceFile) {
		t.Fatalf("content=%q err=%v", content, err)
	}
	items, err := store.ListSkills(ctx, domain.PageOptions{Limit: 10})
	if err != nil || len(items) != 2 || !items[0].SourceFileAvailable || items[1].SourceFileAvailable {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if _, err = store.GetSkillSourceFile(ctx, legacy.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy source err=%v", err)
	}
	if err = store.DeleteSkill(ctx, withFile.ID); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err = store.pool.QueryRow(ctx, `SELECT count(*) FROM skill_source_files WHERE skill_id=$1`, withFile.ID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("remaining=%d err=%v", remaining, err)
	}
}
