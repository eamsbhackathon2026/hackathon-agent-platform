package postgres

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// GetLatestContextSnapshot returns the newest durable summary for a conversation.
func (s *Store) GetLatestContextSnapshot(ctx context.Context, sessionID uuid.UUID) (domain.ContextSnapshot, error) {
	value, err := s.queries(ctx).GetLatestContextSnapshot(ctx, dbID(sessionID))
	if err != nil {
		return domain.ContextSnapshot{}, mapError(err)
	}
	return contextSnapshotModel(value), nil
}

// CreateContextSnapshotCAS inserts only when expectedVersion is still current.
func (s *Store) CreateContextSnapshotCAS(ctx context.Context, value domain.ContextSnapshot, expectedVersion int64) (bool, error) {
	if value.ID == uuid.Nil || value.SessionID == uuid.Nil || value.Version != expectedVersion+1 || value.CoveredThroughSeq < 1 || strings.TrimSpace(value.Summary) == "" {
		return false, domain.ErrValidation
	}
	n, err := s.queries(ctx).CreateContextSnapshotCAS(ctx, sqlcgen.CreateContextSnapshotCASParams{
		ID: dbID(value.ID), SessionID: dbID(value.SessionID), SourceRunID: optionalID(value.SourceRunID),
		Version: value.Version, CoveredThroughSeq: value.CoveredThroughSeq, Summary: value.Summary,
		InputTokens: int64Value(value.Usage.InputTokens), OutputTokens: int64Value(value.Usage.OutputTokens),
		CreatedAt: catalogTime(value.CreatedAt), ExpectedVersion: expectedVersion,
	})
	return n == 1, mapError(err)
}

func contextSnapshotModel(value sqlcgen.SessionContextSnapshot) domain.ContextSnapshot {
	return domain.ContextSnapshot{
		ID: uuid.UUID(value.ID.Bytes), SessionID: uuid.UUID(value.SessionID.Bytes),
		SourceRunID: idPointer(value.SourceRunID), Version: value.Version,
		CoveredThroughSeq: value.CoveredThroughSeq, Summary: value.Summary,
		Usage:     domain.TokenUsage{InputTokens: intPointer(value.InputTokens), OutputTokens: intPointer(value.OutputTokens)},
		CreatedAt: value.CreatedAt.Time.UTC(),
	}
}

var _ outbound.ContextSnapshotRepository = (*Store)(nil)
