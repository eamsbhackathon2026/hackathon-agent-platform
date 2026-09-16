package runs

import (
	"context"
	"errors"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/services/delivery"
)

func (s *Service) finish(parent context.Context, state *executionState, failure *domain.RunFailure, emit func(domain.RunEvent)) (domain.Run, error) {
	now := s.Clock.Now()
	state.run.FinishedAt = &now
	state.run.Usage = state.usage.result()
	var transitionErr error
	if failure == nil {
		transitionErr = state.run.Transition(domain.RunSucceeded)
	} else {
		state.run.Failure = failure
		next := domain.RunFailed
		if failure.Code == "run_cancelled" {
			next = domain.RunCancelled
		}
		transitionErr = state.run.Transition(next)
	}
	if transitionErr != nil {
		return state.run, transitionErr
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()
	// Tool messages are already persisted by the iterations, so the terminal run carries
	// its structured results for the sync response, the completion event and the webhook.
	if err := s.attachToolResults(ctx, &state.run); err != nil {
		return state.run, err
	}
	var persisted domain.Run
	rootStatus := domain.SpanOK
	var errorMessage *string
	if failure != nil {
		rootStatus, errorMessage = domain.SpanError, &failure.Message
	}
	started := state.run.CreatedAt
	if state.run.StartedAt != nil {
		started = *state.run.StartedAt
	}
	var webhook *domain.WebhookDelivery
	if state.run.WebhookURL != nil && state.run.TriggeredByAPIKeyID != nil && s.Deliveries != nil {
		payload, buildErr := delivery.BuildPayload(state.run, now)
		if buildErr != nil {
			return state.run, buildErr
		}
		deliveryID, idErr := s.IDs.NewID()
		if idErr != nil {
			return state.run, idErr
		}
		webhook = &domain.WebhookDelivery{ID: deliveryID, RunID: state.run.ID, APIKeyID: *state.run.TriggeredByAPIKeyID, URL: *state.run.WebhookURL, Event: domain.TerminalWebhookEvent(state.run.Status), Payload: payload, Status: domain.WebhookPending, Attempts: 0, NextAttemptAt: &now, CreatedAt: now}
	}
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var persistErr error
		persisted, persistErr = s.Runs.FinishRun(ctx, state.run)
		if persistErr != nil {
			return persistErr
		}
		if spanErr := s.Spans.CreateSpan(ctx, domain.Span{ID: state.rootSpanID, RunID: persisted.ID, Kind: domain.SpanRun, Name: "run.execute", Status: rootStatus, Usage: persisted.Usage, StartedAt: started, EndedAt: now, DurationMS: durationMS(started, now), Attributes: []byte(`{}`), ErrorMessage: errorMessage}); spanErr != nil {
			return spanErr
		}
		if webhook != nil {
			return s.Deliveries.InsertWebhookDelivery(ctx, *webhook)
		}
		return nil
	})
	if err != nil {
		return persisted, err
	}
	// The repository maps a fresh row, so carry the transcript-derived results over: the
	// sync response, the completion event and the webhook must all describe the same run.
	persisted.ToolResults = state.run.ToolResults
	if failure == nil {
		emit(domain.RunEvent{Type: domain.EventRunCompleted, Run: &persisted})
	} else {
		emit(domain.RunEvent{Type: domain.EventRunFailed, Run: &persisted, Failure: failure})
	}
	return persisted, nil
}

func durationMS(started, ended time.Time) int64 {
	duration := ended.Sub(started).Milliseconds()
	if duration < 0 {
		return 0
	}
	return duration
}

func failureFromCause(err error) *domain.RunFailure {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &domain.RunFailure{Code: "run_timeout", Message: "Lần xử lý đã vượt quá thời gian cho phép."}
	case errors.Is(err, context.Canceled), errors.Is(err, errCancelRequested), errors.Is(err, errEventSink):
		return &domain.RunFailure{Code: "run_cancelled", Message: "Lần xử lý đã được dừng."}
	default:
		return internalFailure()
	}
}

func safeRunFailure(err error) *domain.RunFailure {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrProviderNotConfigured):
		return &domain.RunFailure{Code: "provider_not_configured", Message: "Kết nối AI chưa có đủ thông tin truy cập."}
	case errors.Is(err, domain.ErrProviderAuth):
		return &domain.RunFailure{Code: "provider_auth_failed", Message: "Khóa của kết nối AI không hợp lệ hoặc không có quyền truy cập."}
	case errors.Is(err, domain.ErrModelNotFound):
		return &domain.RunFailure{Code: "model_not_found", Message: "Không tìm thấy mô hình đã cấu hình."}
	case errors.Is(err, domain.ErrProviderRateLimited):
		return &domain.RunFailure{Code: "rate_limited", Message: "Kết nối AI đang giới hạn yêu cầu; hãy thử lại sau."}
	case errors.Is(err, domain.ErrProviderBadRequest):
		return &domain.RunFailure{Code: "validation_failed", Message: "Kết nối AI không chấp nhận yêu cầu."}
	case errors.Is(err, domain.ErrProviderUnreachable), errors.Is(err, domain.ErrEgressDenied):
		return &domain.RunFailure{Code: "provider_unreachable", Message: "Không thể truy cập kết nối AI."}
	default:
		return internalFailure()
	}
}

func internalFailure() *domain.RunFailure {
	return &domain.RunFailure{Code: "internal", Message: "Không thể hoàn tất lần xử lý."}
}
