package runs

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

func TestRunQueriesEnforceOwnershipAndPaginate(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	otherUser, keyID := uuid.New(), uuid.New()
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	owned := seedRun(t, fixture, fixture.principal.UserID, uuid.Nil, now)
	seedRun(t, fixture, otherUser, uuid.Nil, now.Add(time.Second))
	keyRun := seedRun(t, fixture, uuid.Nil, keyID, now.Add(2*time.Second))
	seedRun(t, fixture, fixture.principal.UserID, uuid.Nil, now.Add(3*time.Second))

	first, err := fixture.service.ListRuns(t.Context(), fixture.principal, inbound.RunListRequest{PageRequest: inbound.PageRequest{Limit: 1}})
	if err != nil || len(first.Items) != 1 || first.NextCursor == nil {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := fixture.service.ListRuns(t.Context(), fixture.principal, inbound.RunListRequest{PageRequest: inbound.PageRequest{Limit: 1, Cursor: *first.NextCursor}})
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != owned.ID {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	owner := fixture.principal
	owner.Role = domain.RoleOwner
	all, err := fixture.service.ListRuns(t.Context(), owner, inbound.RunListRequest{})
	if err != nil || len(all.Items) != 4 {
		t.Fatalf("owner items=%d err=%v", len(all.Items), err)
	}
	apiKey := domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyID, Scopes: []string{"runs:read"}}
	keyPage, err := fixture.service.ListRuns(t.Context(), apiKey, inbound.RunListRequest{})
	if err != nil || len(keyPage.Items) != 1 || keyPage.Items[0].ID != keyRun.ID {
		t.Fatalf("key page=%+v err=%v", keyPage, err)
	}
	if _, err = fixture.service.GetRun(t.Context(), fixture.principal, keyRun.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-owner get: %v", err)
	}
	if _, err = fixture.service.ListRunSpans(t.Context(), apiKey, keyRun.ID, inbound.PageRequest{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("API key accessed spans: %v", err)
	}
}

func TestSessionQueriesBlockAPIKeysAndActiveDeletion(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	sessionID := uuid.New()
	userID := fixture.principal.UserID
	if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: sessionID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Chat", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CreateRun(t.Context(), domain.Run{ID: uuid.New(), AgentID: fixture.agent.ID, SessionID: sessionID, Mode: domain.RunModeSync, Status: domain.RunRunning, Source: domain.RunSourcePlayground, TriggeredByUserID: &userID, Input: domain.RunInput{Message: "Hi"}, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.DeleteSession(t.Context(), fixture.principal, sessionID); !errors.Is(err, domain.ErrRunInProgress) {
		t.Fatalf("active deletion: %v", err)
	}
	apiKey := domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: uuid.New(), Scopes: []string{"runs:read"}}
	if _, err := fixture.service.ListSessions(t.Context(), apiKey, inbound.SessionListRequest{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("API key listed sessions: %v", err)
	}
}

func TestSessionListMineScopeAndUpdatedPagination(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	ownerID, otherUserID, apiKeyID := fixture.principal.UserID, uuid.New(), uuid.New()
	ownedOlderID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ownedNewerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	otherID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	apiID := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	for _, session := range []domain.Session{
		{ID: ownedOlderID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &ownerID, Title: "Owned older", CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now},
		{ID: ownedNewerID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &ownerID, Title: "Owned newer", CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
		{ID: otherID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &otherUserID, Title: "Other user", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(time.Hour)},
		{ID: apiID, AgentID: fixture.agent.ID, Source: domain.RunSourceAPI, CreatedByAPIKeyID: &apiKeyID, Title: "API", CreatedAt: now, UpdatedAt: now.Add(2 * time.Hour)},
	} {
		if _, err := fixture.store.CreateSession(t.Context(), session); err != nil {
			t.Fatal(err)
		}
	}

	owner := fixture.principal
	owner.Role = domain.RoleOwner
	workspace, err := fixture.service.ListSessions(t.Context(), owner, inbound.SessionListRequest{})
	if err != nil || len(workspace.Items) != 4 || workspace.Items[0].ID != apiID {
		t.Fatalf("workspace items=%d err=%v", len(workspace.Items), err)
	}

	request := inbound.SessionListRequest{
		PageRequest: inbound.PageRequest{Limit: 1},
		MineOnly:    true,
		Sort:        inbound.SessionSortUpdatedAt,
		Source:      runSourceRef(domain.RunSourcePlayground),
	}
	first, err := fixture.service.ListSessions(t.Context(), owner, request)
	if err != nil || len(first.Items) != 1 || first.Items[0].ID != ownedNewerID || first.NextCursor == nil {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	request.Cursor = *first.NextCursor
	second, err := fixture.service.ListSessions(t.Context(), owner, request)
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != ownedOlderID || second.NextCursor != nil {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	admin := owner
	admin.Role = domain.RoleAdmin
	adminMine, err := fixture.service.ListSessions(t.Context(), admin, inbound.SessionListRequest{MineOnly: true, Sort: inbound.SessionSortUpdatedAt, Source: runSourceRef(domain.RunSourcePlayground)})
	if err != nil || len(adminMine.Items) != 2 || adminMine.Items[0].ID != ownedNewerID || adminMine.Items[1].ID != ownedOlderID {
		t.Fatalf("admin mine=%+v err=%v", adminMine, err)
	}

	memberPage, err := fixture.service.ListSessions(t.Context(), fixture.principal, inbound.SessionListRequest{})
	if err != nil || len(memberPage.Items) != 2 {
		t.Fatalf("member items=%d err=%v", len(memberPage.Items), err)
	}
}

func TestSessionListRejectsInvalidSortAndMismatchedCursor(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	userID := fixture.principal.UserID
	for i := range 2 {
		if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: uuid.New(), AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Chat", CreatedAt: now.Add(time.Duration(i) * time.Hour), UpdatedAt: now.Add(time.Duration(2-i) * time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.service.ListSessions(t.Context(), fixture.principal, inbound.SessionListRequest{Sort: inbound.SessionSort("invalid")}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid sort: %v", err)
	}

	createdPage, err := fixture.service.ListSessions(t.Context(), fixture.principal, inbound.SessionListRequest{PageRequest: inbound.PageRequest{Limit: 1}})
	if err != nil || createdPage.NextCursor == nil {
		t.Fatalf("created page=%+v err=%v", createdPage, err)
	}
	if _, err = fixture.service.ListSessions(t.Context(), fixture.principal, inbound.SessionListRequest{PageRequest: inbound.PageRequest{Limit: 1, Cursor: *createdPage.NextCursor}, Sort: inbound.SessionSortUpdatedAt}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("created cursor used for updated sort: %v", err)
	}

	updatedPage, err := fixture.service.ListSessions(t.Context(), fixture.principal, inbound.SessionListRequest{PageRequest: inbound.PageRequest{Limit: 1}, Sort: inbound.SessionSortUpdatedAt})
	if err != nil || updatedPage.NextCursor == nil {
		t.Fatalf("updated page=%+v err=%v", updatedPage, err)
	}
	if _, err = fixture.service.ListSessions(t.Context(), fixture.principal, inbound.SessionListRequest{PageRequest: inbound.PageRequest{Limit: 1, Cursor: *updatedPage.NextCursor}}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("updated cursor used for created sort: %v", err)
	}
}

func TestSessionMessagesSpansCancellationAndValidationBranches(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	userID := fixture.principal.UserID
	sessionID := uuid.New()
	if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: sessionID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Chat", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for i, role := range []string{"user", "assistant"} {
		if _, err := fixture.store.AppendMessage(t.Context(), domain.Message{ID: uuid.New(), SessionID: sessionID, Role: role, Content: role, ToolCalls: []domain.ToolCall{}, CreatedAt: now.Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := fixture.service.ListSessionMessages(t.Context(), fixture.principal, sessionID, inbound.MessageListRequest{PageRequest: inbound.PageRequest{Limit: 1}})
	if err != nil || len(first.Items) != 1 || first.NextCursor == nil {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := fixture.service.ListSessionMessages(t.Context(), fixture.principal, sessionID, inbound.MessageListRequest{PageRequest: inbound.PageRequest{Limit: 1, Cursor: *first.NextCursor}})
	if err != nil || len(second.Items) != 1 || second.Items[0].Role != "assistant" {
		t.Fatalf("second=%+v err=%v", second, err)
	}

	run := domain.Run{ID: uuid.New(), AgentID: fixture.agent.ID, SessionID: sessionID, Mode: domain.RunModeSync, Status: domain.RunRunning, Source: domain.RunSourcePlayground, TriggeredByUserID: &userID, Input: domain.RunInput{Message: "Hi"}, CreatedAt: now}
	if err = fixture.store.CreateRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	cancelled, err := fixture.service.CancelRun(t.Context(), fixture.principal, run.ID)
	if err != nil || cancelled.CancelRequestedAt == nil {
		t.Fatalf("cancelled=%+v err=%v", cancelled, err)
	}
	for i := range 2 {
		if err = fixture.store.CreateSpan(t.Context(), domain.Span{ID: uuid.New(), RunID: run.ID, Kind: domain.SpanLLMCall, Name: "llm", Status: domain.SpanOK, StartedAt: now.Add(time.Duration(i) * time.Second), EndedAt: now.Add(time.Duration(i) * time.Second), Attributes: json.RawMessage(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	activeSpans, err := fixture.service.ListRunSpans(t.Context(), fixture.principal, run.ID, inbound.PageRequest{})
	if err != nil || len(activeSpans.Items) != 0 {
		t.Fatalf("active spans=%+v err=%v", activeSpans, err)
	}
	run.Status = domain.RunSucceeded
	if _, err = fixture.store.FinishRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	spanFirst, err := fixture.service.ListRunSpans(t.Context(), fixture.principal, run.ID, inbound.PageRequest{Limit: 1})
	if err != nil || len(spanFirst.Items) != 1 || spanFirst.NextCursor == nil {
		t.Fatalf("span first=%+v err=%v", spanFirst, err)
	}
	spanSecond, err := fixture.service.ListRunSpans(t.Context(), fixture.principal, run.ID, inbound.PageRequest{Limit: 1, Cursor: *spanFirst.NextCursor})
	if err != nil || len(spanSecond.Items) != 1 {
		t.Fatalf("span second=%+v err=%v", spanSecond, err)
	}

	if _, err = fixture.service.ListRuns(t.Context(), fixture.principal, inbound.RunListRequest{Status: runStatusRef("bad")}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid status: %v", err)
	}
	from, to := now.Add(time.Hour), now
	if _, err = fixture.service.ListRuns(t.Context(), fixture.principal, inbound.RunListRequest{From: &from, To: &to}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid range: %v", err)
	}
	if _, err = fixture.service.ListSessions(t.Context(), fixture.principal, inbound.SessionListRequest{PageRequest: inbound.PageRequest{Cursor: "bad"}}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid cursor: %v", err)
	}
}

func TestSessionMessagesCanBeScopedToOneRun(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	userID, sessionID := fixture.principal.UserID, uuid.New()
	if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: sessionID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Chat", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	runA, runB := uuid.New(), uuid.New()
	for index, runID := range []uuid.UUID{runA, runB, runA} {
		if _, err := fixture.store.AppendMessage(t.Context(), domain.Message{ID: uuid.New(), SessionID: sessionID, RunID: &runID, Role: "assistant", Content: runID.String(), ToolCalls: []domain.ToolCall{}, CreatedAt: now.Add(time.Duration(index) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}

	request := inbound.MessageListRequest{PageRequest: inbound.PageRequest{Limit: 1}, RunID: &runA}
	first, err := fixture.service.ListSessionMessages(t.Context(), fixture.principal, sessionID, request)
	if err != nil || len(first.Items) != 1 || first.Items[0].RunID == nil || *first.Items[0].RunID != runA || first.NextCursor == nil {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	request.Cursor = *first.NextCursor
	second, err := fixture.service.ListSessionMessages(t.Context(), fixture.principal, sessionID, request)
	if err != nil || len(second.Items) != 1 || second.Items[0].RunID == nil || *second.Items[0].RunID != runA || second.NextCursor != nil {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func runStatusRef(value domain.RunStatus) *domain.RunStatus { return &value }

func runSourceRef(value domain.RunSource) *domain.RunSource { return &value }

func seedRun(t *testing.T, fixture engineFixture, userID, keyID uuid.UUID, created time.Time) domain.Run {
	t.Helper()
	sessionID := uuid.New()
	session := domain.Session{ID: sessionID, AgentID: fixture.agent.ID, Title: "Chat", CreatedAt: created, UpdatedAt: created}
	run := domain.Run{ID: uuid.New(), AgentID: fixture.agent.ID, SessionID: sessionID, Mode: domain.RunModeSync, Status: domain.RunSucceeded, Input: domain.RunInput{Message: "Hi"}, CreatedAt: created}
	if keyID != uuid.Nil {
		session.Source, session.CreatedByAPIKeyID = domain.RunSourceAPI, &keyID
		run.Source, run.TriggeredByAPIKeyID = domain.RunSourceAPI, &keyID
	} else {
		session.Source, session.CreatedByUserID = domain.RunSourcePlayground, &userID
		run.Source, run.TriggeredByUserID = domain.RunSourcePlayground, &userID
	}
	if _, err := fixture.store.CreateSession(t.Context(), session); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CreateRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestGetRunAttachesToolResultsButListDoesNot(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	run := seedRun(t, fixture, fixture.principal.UserID, uuid.Nil, now)
	callID := "call_precheck"
	runRef := run.ID
	for _, message := range []domain.Message{
		{ID: uuid.New(), SessionID: run.SessionID, RunID: &runRef, Role: "assistant", Content: "Để tôi kiểm tra.", ToolCalls: []domain.ToolCall{{ID: callID, Name: "http_precheck_transfer", Arguments: []byte(`{"customer_id":100001}`)}}},
		{ID: uuid.New(), SessionID: run.SessionID, RunID: &runRef, Role: "tool", ToolCallID: &callID, ToolName: ptr("http_precheck_transfer"), Content: `{"level":"intervene","scenario_id":"S09"}`},
	} {
		if _, err := fixture.store.AppendMessage(t.Context(), message); err != nil {
			t.Fatal(err)
		}
	}
	got, err := fixture.service.GetRun(t.Context(), fixture.principal, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ToolResults) != 1 || got.ToolResults[0].ToolName != "http_precheck_transfer" || got.ToolResults[0].CallID != callID || string(got.ToolResults[0].Result) != `{"level":"intervene","scenario_id":"S09"}` {
		t.Fatalf("tool results=%+v", got.ToolResults)
	}
	// The list endpoint is a summary; it must not pay for a transcript read per row.
	page, err := fixture.service.ListRuns(t.Context(), fixture.principal, inbound.RunListRequest{})
	if err != nil || len(page.Items) != 1 || page.Items[0].ToolResults != nil {
		t.Fatalf("list page=%+v err=%v", page, err)
	}
}

func ptr[T any](value T) *T { return &value }
