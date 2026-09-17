package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// CreateRun persists the initial queued or running lifecycle state.
func (s *Store) CreateRun(ctx context.Context, v domain.Run) error {
	iterations, err := runIterations(v.Iterations)
	if err != nil {
		return err
	}
	input, _ := json.Marshal(v.Input)
	metadata := v.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	var code, message *string
	if v.Failure != nil {
		code, message = &v.Failure.Code, &v.Failure.Message
	}
	return mapError(s.queries(ctx).CreateRun(ctx, sqlcgen.CreateRunParams{ID: dbID(v.ID), AgentID: dbID(v.AgentID), SessionID: dbID(v.SessionID), Mode: string(v.Mode), Status: string(v.Status), Source: string(v.Source), TriggeredByUserID: optionalID(v.TriggeredByUserID), TriggeredByApiKeyID: optionalID(v.TriggeredByAPIKeyID), Input: input, Output: textValue(v.Output), ErrorCode: textValue(code), ErrorMessage: textValue(message), Iterations: iterations, InputTokens: int64Value(v.Usage.InputTokens), OutputTokens: int64Value(v.Usage.OutputTokens), Metadata: metadata, CancelRequestedAt: optionalTime(v.CancelRequestedAt), QueuedAt: optionalTime(v.QueuedAt), StartedAt: optionalTime(v.StartedAt), FinishedAt: optionalTime(v.FinishedAt), CreatedAt: catalogTime(v.CreatedAt), WebhookUrl: textValue(v.WebhookURL)}))
}

// GetRun loads one execution by ID.
func (s *Store) GetRun(ctx context.Context, id uuid.UUID) (domain.Run, error) {
	row, err := s.queries(ctx).GetRun(ctx, dbID(id))
	if err != nil {
		return domain.Run{}, mapError(err)
	}
	return runModel(row)
}

// StartQueuedRun atomically gives execution ownership to the first worker.
func (s *Store) StartQueuedRun(ctx context.Context, id uuid.UUID, at time.Time) (domain.Run, error) {
	row, err := s.queries(ctx).StartQueuedRun(ctx, sqlcgen.StartQueuedRunParams{ID: dbID(id), StartedAt: dbTime(at)})
	if err != nil {
		return domain.Run{}, mapError(err)
	}
	return runModel(row)
}

// ListRuns applies ownership, public filters and descending keyset pagination.
func (s *Store) ListRuns(ctx context.Context, p outbound.RunListOptions) ([]domain.Run, error) {
	limit, before, beforeID := page(domain.PageOptions{Limit: p.Limit, Before: p.Before})
	rows, err := s.queries(ctx).ListRuns(ctx, sqlcgen.ListRunsParams{AgentID: optionalID(p.AgentID), SessionID: optionalID(p.SessionID), Status: textValue(p.Status), Source: textValue(p.Source), FromTime: optionalTime(p.From), ToTime: optionalTime(p.To), OwnerUserID: optionalID(p.OwnerUserID), OwnerApiKeyID: optionalID(p.OwnerAPIKeyID), BeforeTime: before, BeforeID: beforeID, Limit: limit})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.Run, 0, len(rows))
	for _, row := range rows {
		run, decodeErr := runModel(row)
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = append(result, run)
	}
	return result, nil
}

// RequestRunCancel atomically records an idempotent cancellation timestamp.
func (s *Store) RequestRunCancel(ctx context.Context, id uuid.UUID, at time.Time) (domain.Run, error) {
	row, err := s.queries(ctx).RequestRunCancel(ctx, sqlcgen.RequestRunCancelParams{ID: dbID(id), CancelRequestedAt: catalogTime(at)})
	if err != nil {
		return domain.Run{}, mapError(err)
	}
	return runModel(sqlcgen.Run(row))
}

// RunCancelRequested reports the durable cancellation flag.
func (s *Store) RunCancelRequested(ctx context.Context, id uuid.UUID) (bool, error) {
	requested, err := s.queries(ctx).RunCancelRequested(ctx, dbID(id))
	return requested, mapError(err)
}

// FinishRun changes an active execution to its supplied terminal state.
func (s *Store) FinishRun(ctx context.Context, v domain.Run) (domain.Run, error) {
	iterations, conversionErr := runIterations(v.Iterations)
	if conversionErr != nil {
		return domain.Run{}, conversionErr
	}
	var code, message *string
	if v.Failure != nil {
		code, message = &v.Failure.Code, &v.Failure.Message
	}
	row, err := s.queries(ctx).FinishRun(ctx, sqlcgen.FinishRunParams{ID: dbID(v.ID), Status: string(v.Status), Output: textValue(v.Output), ErrorCode: textValue(code), ErrorMessage: textValue(message), Iterations: iterations, InputTokens: int64Value(v.Usage.InputTokens), OutputTokens: int64Value(v.Usage.OutputTokens), FinishedAt: optionalTime(v.FinishedAt)})
	if err != nil {
		return domain.Run{}, mapError(err)
	}
	return runModel(row)
}

func runIterations(value int) (int32, error) {
	if value < 0 || value > 25 {
		return 0, domain.ErrValidation
	}
	return int32(value), nil
}

var _ outbound.RunRepository = (*Store)(nil)
