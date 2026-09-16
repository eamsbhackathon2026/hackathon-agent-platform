package runs

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// GetSession returns a visible conversation to JWT principals only.
func (s *Service) GetSession(ctx context.Context, p domain.Principal, id uuid.UUID) (domain.Session, error) {
	if p.Kind != domain.PrincipalUser {
		return domain.Session{}, domain.ErrForbidden
	}
	session, err := s.Sessions.GetSession(ctx, id)
	if err != nil {
		return domain.Session{}, err
	}
	if err = domain.Authorize(p, domain.ActionSessionsRead, session.OwnerID()); err != nil {
		return domain.Session{}, err
	}
	return session, nil
}

// ListSessions filters member reads to conversations they created.
func (s *Service) ListSessions(ctx context.Context, p domain.Principal, request inbound.SessionListRequest) (page inbound.SessionPage, err error) {
	if p.Kind != domain.PrincipalUser {
		return page, domain.ErrForbidden
	}
	if err = domain.Authorize(p, domain.ActionSessionsRead, &p.UserID); err != nil {
		return
	}
	if request.Source != nil && *request.Source != domain.RunSourceAPI && *request.Source != domain.RunSourcePlayground {
		return page, domain.Invalid("source", "Nguồn hội thoại không hợp lệ.")
	}
	sort := request.Sort
	if sort == "" {
		sort = inbound.SessionSortCreatedAt
	}
	filter := outbound.SessionListOptions{AgentID: request.AgentID, Source: request.Source}
	switch sort {
	case inbound.SessionSortCreatedAt:
		options, pageErr := descendingPage(request.PageRequest)
		if pageErr != nil {
			return page, pageErr
		}
		filter.Limit, filter.Before = options.Limit, options.Before
	case inbound.SessionSortUpdatedAt:
		filter.Limit, filter.BeforeUpdated, err = updatedSessionPage(request.PageRequest)
		if err != nil {
			return page, err
		}
		filter.OrderByUpdatedAt = true
	default:
		return page, domain.Invalid("sort", "Thứ tự hội thoại không hợp lệ.")
	}
	if request.MineOnly || p.Role == domain.RoleMember {
		filter.OwnerUserID = &p.UserID
	}
	page.Items, err = s.Sessions.ListSessions(ctx, filter)
	if err != nil {
		return page, err
	}
	if len(page.Items) == filter.Limit {
		page.Items = page.Items[:len(page.Items)-1]
		last := page.Items[len(page.Items)-1]
		if filter.OrderByUpdatedAt {
			page.NextCursor = encodeCursor(outbound.SessionUpdatedCursor{UpdatedAt: last.UpdatedAt, ID: last.ID})
		} else {
			page.NextCursor = encodeCursor(domain.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		}
	}
	return page, nil
}

// DeleteSession hides an idle conversation while preserving run history.
func (s *Service) DeleteSession(ctx context.Context, p domain.Principal, id uuid.UUID) error {
	if p.Kind != domain.PrincipalUser {
		return domain.ErrForbidden
	}
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		session, err := s.Sessions.GetSessionForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if err = domain.Authorize(p, domain.ActionSessionsRead, session.OwnerID()); err != nil {
			return err
		}
		active, err := s.Sessions.HasActiveRuns(ctx, session.ID)
		if err != nil {
			return err
		}
		if active {
			return &domain.Error{Kind: domain.ErrRunInProgress, Detail: "Hãy dừng hoặc chờ lần xử lý hiện tại hoàn tất."}
		}
		return s.Sessions.DeleteSession(ctx, session.ID, s.Clock.Now())
	})
}

// ListSessionMessages returns ordered public messages without provider metadata.
func (s *Service) ListSessionMessages(ctx context.Context, p domain.Principal, sessionID uuid.UUID, request inbound.MessageListRequest) (page inbound.MessagePage, err error) {
	if _, err = s.GetSession(ctx, p, sessionID); err != nil {
		return page, err
	}
	limit, after, err := messagePage(request.PageRequest)
	if err != nil {
		return page, err
	}
	if request.RunID == nil {
		page.Items, err = s.Messages.ListMessages(ctx, sessionID, limit, after)
	} else {
		page.Items, err = s.Messages.ListRunMessages(ctx, sessionID, *request.RunID, limit, after)
	}
	if err != nil {
		return page, err
	}
	if len(page.Items) == limit {
		page.Items = page.Items[:len(page.Items)-1]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(outbound.MessageCursor{Seq: last.Seq, ID: last.ID})
	}
	return page, nil
}
