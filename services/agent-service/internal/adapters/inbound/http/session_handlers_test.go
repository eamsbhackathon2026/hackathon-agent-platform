package http

import (
	"context"
	"testing"

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
}

func (s *sessionQuerySpy) ListSessions(_ context.Context, principal domain.Principal, request inbound.SessionListRequest) (inbound.SessionPage, error) {
	s.principal = principal
	s.request = request
	return inbound.SessionPage{}, nil
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
