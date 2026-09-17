package http

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

type sessionQuerySpy struct {
	inbound.SessionQueryUseCase
	principal     domain.Principal
	request       inbound.SessionListRequest
	messageFilter inbound.MessageListRequest
	page          inbound.SessionPage
}

func (s *sessionQuerySpy) ListSessions(_ context.Context, principal domain.Principal, request inbound.SessionListRequest) (inbound.SessionPage, error) {
	s.principal = principal
	s.request = request
	return s.page, nil
}

func (s *sessionQuerySpy) ListSessionMessages(_ context.Context, principal domain.Principal, _ uuid.UUID, request inbound.MessageListRequest) (inbound.MessagePage, error) {
	s.principal = principal
	s.messageFilter = request
	return inbound.MessagePage{}, nil
}

func TestSessionHandlerMapsMineScopeAndUpdatedSort(t *testing.T) {
	spy := &sessionQuerySpy{}
	handler := NewSessionHandler(spy)
	principal := domain.Principal{Kind: domain.PrincipalUser, UserID: uuid.New(), Role: domain.RoleOwner}
	agentID := uuid.New()
	limit, cursor := 12, "opaque"
	source, scope, sort := gen.Playground, gen.Mine, gen.UpdatedAt
	ctx := context.WithValue(t.Context(), requestContextKey{}, requestInfo{principal: principal})

	response, err := handler.ListSessions(ctx, gen.ListSessionsRequestObject{Params: gen.ListSessionsParams{
		Limit: &limit, Cursor: &cursor, AgentId: &agentID, Source: &source, Scope: &scope, Sort: &sort,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(gen.ListSessions200JSONResponse); !ok {
		t.Fatalf("response=%T", response)
	}
	if spy.principal.Kind != principal.Kind || spy.principal.UserID != principal.UserID || spy.principal.Role != principal.Role || spy.request.Limit != limit || spy.request.Cursor != cursor || spy.request.AgentID == nil || *spy.request.AgentID != agentID || spy.request.Source == nil || *spy.request.Source != domain.RunSourcePlayground || !spy.request.MineOnly || spy.request.Sort != inbound.SessionSortUpdatedAt {
		t.Fatalf("principal=%+v request=%+v", spy.principal, spy.request)
	}
}

func TestSessionHandlerMapsRunMessageFilter(t *testing.T) {
	spy := &sessionQuerySpy{}
	handler := NewSessionHandler(spy)
	principal := domain.Principal{Kind: domain.PrincipalUser, UserID: uuid.New(), Role: domain.RoleMember}
	sessionID, runID := uuid.New(), uuid.New()
	limit, cursor := 20, "opaque"
	ctx := context.WithValue(t.Context(), requestContextKey{}, requestInfo{principal: principal})

	response, err := handler.ListSessionMessages(ctx, gen.ListSessionMessagesRequestObject{
		SessionId: sessionID,
		Params:    gen.ListSessionMessagesParams{Limit: &limit, Cursor: &cursor, RunId: &runID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(gen.ListSessionMessages200JSONResponse); !ok {
		t.Fatalf("response=%T", response)
	}
	if spy.principal.UserID != principal.UserID || spy.messageFilter.Limit != limit || spy.messageFilter.Cursor != cursor || spy.messageFilter.RunID == nil || *spy.messageFilter.RunID != runID {
		t.Fatalf("principal=%+v filter=%+v", spy.principal, spy.messageFilter)
	}
}

func TestSessionHandlerMapsActivityWindowAndSummary(t *testing.T) {
	sessionID, runID := uuid.New(), uuid.New()
	status, role, preview, input := domain.RunFailed, "assistant", "Latest answer", 7
	spy := &sessionQuerySpy{page: inbound.SessionPage{
		Items:     []domain.Session{{ID: sessionID, AgentID: uuid.New(), Source: domain.RunSourceAPI}},
		Summaries: map[uuid.UUID]domain.SessionSummary{sessionID: {TurnCount: 3, FailedTurnCount: 1, LatestRunID: &runID, LatestRunStatus: &status, Usage: domain.TokenUsage{InputTokens: &input}, ProcessingMS: 900, LastMessage: &preview, LastMessageRole: &role}},
	}}
	handler := NewSessionHandler(spy)
	principal := domain.Principal{Kind: domain.PrincipalUser, UserID: uuid.New(), Role: domain.RoleOwner}
	ctx := context.WithValue(t.Context(), requestContextKey{}, requestInfo{principal: principal})
	from, to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	response, err := handler.ListSessions(ctx, gen.ListSessionsRequestObject{Params: gen.ListSessionsParams{From: &from, To: &to}})
	if err != nil {
		t.Fatal(err)
	}
	if spy.request.From == nil || !spy.request.From.Equal(from) || spy.request.To == nil || !spy.request.To.Equal(to) {
		t.Fatalf("request=%+v", spy.request)
	}
	page, ok := response.(gen.ListSessions200JSONResponse)
	if !ok || len(page.Items) != 1 || page.Items[0].Summary == nil {
		t.Fatalf("response=%+v", response)
	}
	summary := page.Items[0].Summary
	gotStatus, _ := summary.LatestRunStatus.Get()
	gotRole, _ := summary.LastMessageRole.Get()
	gotRun, _ := summary.LatestRunId.Get()
	gotInput, _ := summary.Usage.InputTokens.Get()
	if summary.TurnCount != 3 || summary.FailedTurnCount != 1 || gotStatus != gen.SessionSummaryLatestRunStatus("failed") || gotRole != gen.SessionSummaryLastMessageRole("assistant") || gotRun != runID || gotInput != 7 || !summary.Usage.OutputTokens.IsNull() || !summary.FirstMessage.IsNull() || summary.ProcessingMs != 900 {
		t.Fatalf("summary=%+v", summary)
	}
}
