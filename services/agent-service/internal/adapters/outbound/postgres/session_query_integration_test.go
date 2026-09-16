//go:build integration

package postgres

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

func TestSessionListByUpdatedAtFiltersPaginatesAndUsesActivityTime(t *testing.T) {
	store, provider, agent := catalogFixture(t)
	ctx := t.Context()
	if err := store.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	ownerID := agent.CreatedBy
	otherUserID := uuid.New()
	if err := store.CreateUser(ctx, domain.User{ID: otherUserID, Email: "session-other@example.test", Name: "Other", PasswordHash: "hash", Role: domain.RoleMember, Status: domain.UserActive, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	ownedOlderID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ownedSameLowID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	ownedSameHighID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	otherID := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	for _, session := range []domain.Session{
		{ID: ownedOlderID, AgentID: agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &ownerID, Title: "Older", CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour)},
		{ID: ownedSameLowID, AgentID: agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &ownerID, Title: "Same low", CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
		{ID: ownedSameHighID, AgentID: agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &ownerID, Title: "Same high", CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
		{ID: otherID, AgentID: agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &otherUserID, Title: "Other", CreatedAt: now, UpdatedAt: now.Add(10 * time.Hour)},
	} {
		if _, err := store.CreateSession(ctx, session); err != nil {
			t.Fatal(err)
		}
	}

	activityTime := now.Add(5 * time.Hour)
	if _, err := store.AppendMessage(ctx, domain.Message{ID: uuid.New(), SessionID: ownedOlderID, Role: "user", Content: "Continue", ToolCalls: []domain.ToolCall{}, CreatedAt: activityTime}); err != nil {
		t.Fatal(err)
	}
	source := domain.RunSourcePlayground
	options := outbound.SessionListOptions{Limit: 2, OwnerUserID: &ownerID, Source: &source, OrderByUpdatedAt: true}
	first, err := store.ListSessions(ctx, options)
	if err != nil || len(first) != 2 || first[0].ID != ownedOlderID || !first[0].UpdatedAt.Equal(activityTime) || first[1].ID != ownedSameHighID {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	options.BeforeUpdated = &outbound.SessionUpdatedCursor{UpdatedAt: first[1].UpdatedAt, ID: first[1].ID}
	second, err := store.ListSessions(ctx, options)
	if err != nil || len(second) != 1 || second[0].ID != ownedSameLowID {
		t.Fatalf("second=%+v err=%v", second, err)
	}

	if err = store.DeleteSession(ctx, ownedSameLowID, activityTime.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	options.BeforeUpdated = nil
	remaining, err := store.ListSessions(ctx, options)
	if err != nil || len(remaining) != 2 || remaining[0].ID != ownedOlderID || remaining[1].ID != ownedSameHighID {
		t.Fatalf("remaining=%+v err=%v", remaining, err)
	}

	var indexExists bool
	if err = store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname='sessions_user_source_updated')`).Scan(&indexExists); err != nil || !indexExists {
		t.Fatalf("recent index exists=%v err=%v", indexExists, err)
	}
}
