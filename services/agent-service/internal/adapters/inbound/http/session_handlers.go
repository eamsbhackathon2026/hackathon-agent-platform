package http

import (
	"context"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// SessionHandler exposes JWT-only conversation history.
type SessionHandler struct{ sessions inbound.SessionQueryUseCase }

// NewSessionHandler binds conversation queries to their use case.
func NewSessionHandler(sessions inbound.SessionQueryUseCase) *SessionHandler {
	return &SessionHandler{sessions: sessions}
}

// ListSessions returns one authorized conversation page.
func (h *SessionHandler) ListSessions(ctx context.Context, request gen.ListSessionsRequestObject) (gen.ListSessionsResponseObject, error) {
	filter := inbound.SessionListRequest{PageRequest: pageRequest(request.Params.Limit, request.Params.Cursor), AgentID: request.Params.AgentId, From: request.Params.From, To: request.Params.To}
	if request.Params.Source != nil {
		value := domain.RunSource(*request.Params.Source)
		filter.Source = &value
	}
	filter.MineOnly = request.Params.Scope != nil && *request.Params.Scope == gen.Mine
	if request.Params.Sort != nil {
		filter.Sort = inbound.SessionSort(*request.Params.Sort)
	}
	page, err := h.sessions.ListSessions(ctx, requestFrom(ctx).principal, filter)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Session, len(page.Items))
	for i, session := range page.Items {
		items[i] = sessionDTO(session)
		if summary, ok := page.Summaries[session.ID]; ok {
			dto := sessionSummaryDTO(summary)
			items[i].Summary = &dto
		}
	}
	return gen.ListSessions200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// GetSession returns one authorized conversation.
func (h *SessionHandler) GetSession(ctx context.Context, request gen.GetSessionRequestObject) (gen.GetSessionResponseObject, error) {
	session, err := h.sessions.GetSession(ctx, requestFrom(ctx).principal, request.SessionId)
	if err != nil {
		return nil, err
	}
	return gen.GetSession200JSONResponse(sessionDTO(session)), nil
}

// DeleteSession soft-deletes an idle conversation.
func (h *SessionHandler) DeleteSession(ctx context.Context, request gen.DeleteSessionRequestObject) (gen.DeleteSessionResponseObject, error) {
	err := h.sessions.DeleteSession(ctx, requestFrom(ctx).principal, request.SessionId)
	return gen.DeleteSession204Response{}, err
}

// ListSessionMessages returns public messages without replay metadata.
func (h *SessionHandler) ListSessionMessages(ctx context.Context, request gen.ListSessionMessagesRequestObject) (gen.ListSessionMessagesResponseObject, error) {
	filter := inbound.MessageListRequest{PageRequest: pageRequest(request.Params.Limit, request.Params.Cursor), RunID: request.Params.RunId}
	page, err := h.sessions.ListSessionMessages(ctx, requestFrom(ctx).principal, request.SessionId, filter)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Message, 0, len(page.Items))
	for _, message := range page.Items {
		dto, mapErr := messageDTO(message)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, dto)
	}
	return gen.ListSessionMessages200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}
