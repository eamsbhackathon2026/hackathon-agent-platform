//go:build integration

package postgres

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

func TestSessionSummariesCountRunsAndPreviewMessages(t *testing.T) {
	store, agent, sessionID := reportFixture(t)
	ctx := t.Context()
	day := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	started, finished := day, day.Add(1500*time.Millisecond)
	in, out := 30, 12
	seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunSucceeded, Source: domain.RunSourcePlayground, CreatedAt: day, StartedAt: &started, FinishedAt: &finished, Usage: domain.TokenUsage{InputTokens: &in, OutputTokens: &out}})
	latest := seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunFailed, Source: domain.RunSourcePlayground, CreatedAt: day.Add(time.Hour), Failure: &domain.RunFailure{Code: "tool_failed", Message: "boom"}})
	callID, toolName := "call-1", "lookup"
	for i, message := range []domain.Message{
		{Role: "user", Content: "   "},
		{Role: "user", Content: "First question"},
		{Role: "assistant", Content: "", ToolCalls: []domain.ToolCall{{ID: callID, Name: toolName, Arguments: []byte(`{}`)}}},
		{Role: "tool", Content: "tool output", ToolCallID: &callID, ToolName: &toolName},
		{Role: "assistant", Content: "Latest answer"},
	} {
		message.ID, message.SessionID, message.CreatedAt = uuid.New(), sessionID, day.Add(time.Duration(i)*time.Minute)
		if message.ToolCalls == nil {
			message.ToolCalls = []domain.ToolCall{}
		}
		if _, err := store.AppendMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	emptyID := uuid.New()
	if _, err := store.CreateSession(ctx, domain.Session{ID: emptyID, AgentID: agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &agent.CreatedBy, Title: "Empty", CreatedAt: day, UpdatedAt: day}); err != nil {
		t.Fatal(err)
	}

	summaries, err := store.SummarizeSessions(ctx, []uuid.UUID{sessionID, emptyID})
	if err != nil {
		t.Fatal(err)
	}
	busy := summaries[sessionID]
	if busy.TurnCount != 2 || busy.FailedTurnCount != 1 || busy.LatestRunID == nil || *busy.LatestRunID != latest.ID || busy.LatestRunStatus == nil || *busy.LatestRunStatus != domain.RunFailed || busy.ProcessingMS != 1500 {
		t.Fatalf("busy=%+v", busy)
	}
	if busy.Usage.InputTokens == nil || *busy.Usage.InputTokens != 30 || busy.Usage.OutputTokens == nil || *busy.Usage.OutputTokens != 12 {
		t.Fatalf("usage=%+v", busy.Usage)
	}
	if busy.FirstMessage == nil || *busy.FirstMessage != "First question" || busy.LastMessage == nil || *busy.LastMessage != "Latest answer" || busy.LastMessageRole == nil || *busy.LastMessageRole != "assistant" {
		t.Fatalf("previews=%+v", busy)
	}
	empty, ok := summaries[emptyID]
	if !ok || empty.TurnCount != 0 || empty.LatestRunStatus != nil || empty.Usage.InputTokens != nil || empty.FirstMessage != nil || empty.LastMessage != nil {
		t.Fatalf("empty=%+v ok=%v", empty, ok)
	}

	sessionRuns, err := store.ListRuns(ctx, outbound.RunListOptions{Limit: 10, SessionID: &emptyID})
	if err != nil || len(sessionRuns) != 0 {
		t.Fatalf("empty session runs=%d err=%v", len(sessionRuns), err)
	}
	sessionRuns, err = store.ListRuns(ctx, outbound.RunListOptions{Limit: 10, SessionID: &sessionID})
	if err != nil || len(sessionRuns) != 2 {
		t.Fatalf("session runs=%d err=%v", len(sessionRuns), err)
	}
}

func TestSessionListFiltersByActivityWindow(t *testing.T) {
	store, agent, _ := reportFixture(t)
	ctx := t.Context()
	day := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	insideID, edgeID := uuid.New(), uuid.New()
	for _, session := range []domain.Session{
		{ID: insideID, UpdatedAt: day},
		{ID: edgeID, UpdatedAt: day.Add(time.Hour)},
	} {
		session.AgentID, session.Source, session.CreatedByUserID, session.Title, session.CreatedAt = agent.ID, domain.RunSourcePlayground, &agent.CreatedBy, "Window", day.Add(-time.Hour)
		if _, err := store.CreateSession(ctx, session); err != nil {
			t.Fatal(err)
		}
	}
	from, to := day, day.Add(time.Hour)
	for _, byUpdated := range []bool{false, true} {
		items, err := store.ListSessions(ctx, outbound.SessionListOptions{Limit: 10, OrderByUpdatedAt: byUpdated, UpdatedFrom: &from, UpdatedTo: &to})
		if err != nil || len(items) != 1 || items[0].ID != insideID {
			t.Fatalf("byUpdated=%v items=%+v err=%v", byUpdated, items, err)
		}
	}
}
