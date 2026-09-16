//go:build integration

package postgres

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestContextSnapshotCASPaginationAndCalibration(t *testing.T) {
	store, provider, agent := catalogFixture(t)
	ctx := t.Context()
	if err := store.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := agent.CreatedBy
	session := domain.Session{ID: uuid.New(), AgentID: agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Memory", CreatedAt: now, UpdatedAt: now}
	if _, err := store.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 5; index++ {
		if _, err := store.AppendMessage(ctx, domain.Message{ID: uuid.New(), SessionID: session.ID, Role: "user", Content: "message", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.ListMessagesAfterSeq(ctx, session.ID, 2, 2)
	if err != nil || len(page) != 2 || page[0].Seq != 3 || page[1].Seq != 4 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if err = store.UpdatePromptTokenCalibration(ctx, session.ID, 120, 150, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	savedSession, err := store.GetSession(ctx, session.ID)
	if err != nil || savedSession.LastPromptEstimatedTokens == nil || *savedSession.LastPromptEstimatedTokens != 120 || savedSession.LastPromptTokens == nil || *savedSession.LastPromptTokens != 150 {
		t.Fatalf("session=%+v err=%v", savedSession, err)
	}
	input, output := 40, 10
	first := domain.ContextSnapshot{ID: uuid.New(), SessionID: session.ID, Version: 1, CoveredThroughSeq: 2, Summary: "first", Usage: domain.TokenUsage{InputTokens: &input, OutputTokens: &output}, CreatedAt: now}
	created, err := store.CreateContextSnapshotCAS(ctx, first, 0)
	if err != nil || !created {
		t.Fatalf("first created=%v err=%v", created, err)
	}
	stale := first
	stale.ID, stale.Version, stale.CoveredThroughSeq = uuid.New(), 1, 3
	if created, err = store.CreateContextSnapshotCAS(ctx, stale, 0); err != nil || created {
		t.Fatalf("stale created=%v err=%v", created, err)
	}
	second := first
	second.ID, second.Version, second.CoveredThroughSeq, second.Summary = uuid.New(), 2, 4, "second"
	if created, err = store.CreateContextSnapshotCAS(ctx, second, 1); err != nil || !created {
		t.Fatalf("second created=%v err=%v", created, err)
	}
	latest, err := store.GetLatestContextSnapshot(ctx, session.ID)
	if err != nil || latest.Version != 2 || latest.CoveredThroughSeq != 4 || latest.Summary != "second" {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
}
