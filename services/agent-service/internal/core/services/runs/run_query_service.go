package runs

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// GetRun enforces ownership after loading the durable owner identifiers.
func (s *Service) GetRun(ctx context.Context, p domain.Principal, id uuid.UUID) (domain.Run, error) {
	run, err := s.Runs.GetRun(ctx, id)
	if err != nil {
		return domain.Run{}, err
	}
	if err = domain.Authorize(p, domain.ActionRunsRead, run.OwnerID()); err != nil {
		return domain.Run{}, err
	}
	if err = s.attachToolResults(ctx, &run); err != nil {
		return domain.Run{}, err
	}
	return run, nil
}

// CancelRun records an idempotent request; the engine observes it within CancelPoll.
func (s *Service) CancelRun(ctx context.Context, p domain.Principal, id uuid.UUID) (domain.Run, error) {
	run, err := s.GetRun(ctx, p, id)
	if err != nil || run.Terminal() {
		return run, err
	}
	return s.Runs.RequestRunCancel(ctx, id, s.Clock.Now())
}

// ListRuns applies public filters and principal ownership before pagination.
func (s *Service) ListRuns(ctx context.Context, p domain.Principal, request inbound.RunListRequest) (page inbound.RunPage, err error) {
	owner := p.UserID
	if p.Kind == domain.PrincipalAPIKey {
		owner = p.APIKeyID
	}
	if err = domain.Authorize(p, domain.ActionRunsRead, &owner); err != nil {
		return
	}
	if request.Status != nil && !validRunStatus(*request.Status) {
		return page, domain.Invalid("status", "Trạng thái lần xử lý không hợp lệ.")
	}
	if request.Source != nil && *request.Source != domain.RunSourceAPI && *request.Source != domain.RunSourcePlayground {
		return page, domain.Invalid("source", "Nguồn lần xử lý không hợp lệ.")
	}
	if request.From != nil && request.To != nil && !request.From.Before(*request.To) {
		return page, domain.Invalid("from", "Thời điểm bắt đầu phải trước thời điểm kết thúc.")
	}
	options, err := descendingPage(request.PageRequest)
	if err != nil {
		return page, err
	}
	filter := outbound.RunListOptions{Limit: options.Limit, Before: options.Before, AgentID: request.AgentID, Status: request.Status, Source: request.Source, From: request.From, To: request.To}
	if p.Kind == domain.PrincipalAPIKey {
		filter.OwnerAPIKeyID = &p.APIKeyID
	} else if p.Role == domain.RoleMember {
		filter.OwnerUserID = &p.UserID
	}
	page.Items, err = s.Runs.ListRuns(ctx, filter)
	if err != nil {
		return page, err
	}
	if len(page.Items) == options.Limit {
		page.Items = page.Items[:len(page.Items)-1]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(domain.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, nil
}

// ListRunSpans returns public-safe trace steps to authorized users only.
func (s *Service) ListRunSpans(ctx context.Context, p domain.Principal, runID uuid.UUID, request inbound.PageRequest) (page inbound.SpanPage, err error) {
	run, err := s.Runs.GetRun(ctx, runID)
	if err != nil {
		return page, err
	}
	if err = domain.Authorize(p, domain.ActionSpansRead, run.OwnerID()); err != nil {
		return page, err
	}
	limit, after, err := spanPage(request)
	if err != nil {
		return page, err
	}
	// The root span is inserted atomically with the terminal run update. Hiding
	// partial traces keeps ascending keyset cursors stable while a run is active.
	if !run.Terminal() {
		return page, nil
	}
	page.Items, err = s.Spans.ListSpans(ctx, runID, limit, after)
	if err != nil {
		return page, err
	}
	if len(page.Items) == limit {
		page.Items = page.Items[:len(page.Items)-1]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(outbound.SpanCursor{StartedAt: last.StartedAt, ID: last.ID})
	}
	return page, nil
}

func validRunStatus(status domain.RunStatus) bool {
	return status == domain.RunQueued || status == domain.RunRunning || status == domain.RunSucceeded || status == domain.RunFailed || status == domain.RunCancelled
}
