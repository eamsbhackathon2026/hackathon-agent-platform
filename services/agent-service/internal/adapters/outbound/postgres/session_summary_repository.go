package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"agent-platform/services/agent-service/internal/core/domain"
)

// SummarizeSessions condenses runs and message previews for one listing page. It
// runs three set-based queries regardless of page size.
func (s *Store) SummarizeSessions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.SessionSummary, error) {
	result := make(map[uuid.UUID]domain.SessionSummary, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	keys := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		keys[i] = dbID(id)
		result[id] = domain.SessionSummary{}
	}
	queries := s.queries(ctx)
	runs, err := queries.SessionRunSummaries(ctx, keys)
	if err != nil {
		return nil, mapError(err)
	}
	for _, row := range runs {
		id := uuid.UUID(row.SessionID.Bytes)
		summary := result[id]
		summary.TurnCount, summary.FailedTurnCount = int(row.TurnCount), int(row.FailedTurnCount)
		summary.LatestRunID = idPointer(row.LatestRunID)
		status := domain.RunStatus(row.LatestRunStatus)
		summary.LatestRunStatus = &status
		summary.Usage = domain.TokenUsage{InputTokens: reportedTokens(row.InputTokens, row.InputReported), OutputTokens: reportedTokens(row.OutputTokens, row.OutputReported)}
		summary.ProcessingMS = row.ProcessingMs
		result[id] = summary
	}
	firsts, err := queries.SessionFirstUserMessages(ctx, keys)
	if err != nil {
		return nil, mapError(err)
	}
	for _, row := range firsts {
		id := uuid.UUID(row.SessionID.Bytes)
		summary, content := result[id], row.Content
		summary.FirstMessage = &content
		result[id] = summary
	}
	lasts, err := queries.SessionLastMessages(ctx, keys)
	if err != nil {
		return nil, mapError(err)
	}
	for _, row := range lasts {
		id := uuid.UUID(row.SessionID.Bytes)
		summary, content, role := result[id], row.Content, row.Role
		summary.LastMessage, summary.LastMessageRole = &content, &role
		result[id] = summary
	}
	return result, nil
}

// reportedTokens keeps "no provider reported usage" distinct from a real zero.
func reportedTokens(total, reported int64) *int {
	if reported == 0 {
		return nil
	}
	value := int(total)
	return &value
}
