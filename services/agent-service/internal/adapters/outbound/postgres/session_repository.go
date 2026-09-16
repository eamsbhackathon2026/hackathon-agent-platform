package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// CreateSession inserts a conversation or reports an external-key conflict.
func (s *Store) CreateSession(ctx context.Context, v domain.Session) (bool, error) {
	n, err := s.queries(ctx).CreateSession(ctx, sqlcgen.CreateSessionParams{ID: dbID(v.ID), AgentID: dbID(v.AgentID), Source: string(v.Source), CreatedByUserID: optionalID(v.CreatedByUserID), CreatedByApiKeyID: optionalID(v.CreatedByAPIKeyID), ExternalKey: textValue(v.ExternalKey), Title: v.Title, CreatedAt: catalogTime(v.CreatedAt), UpdatedAt: catalogTime(v.UpdatedAt)})
	return n > 0, mapError(err)
}

// GetSession returns one non-deleted conversation.
func (s *Store) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	v, err := s.queries(ctx).GetSession(ctx, dbID(id))
	return sessionModel(v), mapError(err)
}

// GetSessionForUpdate locks one non-deleted conversation in the current transaction.
func (s *Store) GetSessionForUpdate(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	v, err := s.queries(ctx).GetSessionForUpdate(ctx, dbID(id))
	return sessionModel(v), mapError(err)
}

// GetSessionByExternalKey locks an API-owned conversation for safe continuation.
func (s *Store) GetSessionByExternalKey(ctx context.Context, keyID uuid.UUID, externalKey string) (domain.Session, error) {
	v, err := s.queries(ctx).GetSessionByExternalKey(ctx, sqlcgen.GetSessionByExternalKeyParams{CreatedByApiKeyID: dbID(keyID), ExternalKey: pgtype.Text{String: externalKey, Valid: true}})
	return sessionModel(v), mapError(err)
}

// ListSessions applies ownership, public filters and descending pagination.
func (s *Store) ListSessions(ctx context.Context, p outbound.SessionListOptions) ([]domain.Session, error) {
	limit, before, beforeID := page(domain.PageOptions{Limit: p.Limit, Before: p.Before})
	var rows []sqlcgen.Session
	var err error
	if p.OrderByUpdatedAt {
		updatedBefore, updatedBeforeID := optionalSessionUpdatedCursor(p.BeforeUpdated)
		rows, err = s.queries(ctx).ListSessionsByUpdatedAt(ctx, sqlcgen.ListSessionsByUpdatedAtParams{AgentID: optionalID(p.AgentID), Source: textValue(p.Source), OwnerUserID: optionalID(p.OwnerUserID), OwnerApiKeyID: optionalID(p.OwnerAPIKeyID), BeforeUpdatedAt: updatedBefore, BeforeID: updatedBeforeID, Limit: limit})
	} else {
		rows, err = s.queries(ctx).ListSessions(ctx, sqlcgen.ListSessionsParams{AgentID: optionalID(p.AgentID), Source: textValue(p.Source), OwnerUserID: optionalID(p.OwnerUserID), OwnerApiKeyID: optionalID(p.OwnerAPIKeyID), BeforeTime: before, BeforeID: beforeID, Limit: limit})
	}
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		result = append(result, sessionModel(row))
	}
	return result, nil
}

func optionalSessionUpdatedCursor(cursor *outbound.SessionUpdatedCursor) (pgtype.Timestamptz, pgtype.UUID) {
	if cursor == nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}
	}
	return catalogTime(cursor.UpdatedAt), dbID(cursor.ID)
}

// HasActiveRuns checks whether deletion would hide an active execution.
func (s *Store) HasActiveRuns(ctx context.Context, id uuid.UUID) (bool, error) {
	result, err := s.queries(ctx).SessionHasActiveRuns(ctx, dbID(id))
	return result, mapError(err)
}

// DeleteSession soft-deletes an idle conversation.
func (s *Store) DeleteSession(ctx context.Context, id uuid.UUID, at time.Time) error {
	n, err := s.queries(ctx).DeleteSession(ctx, sqlcgen.DeleteSessionParams{ID: dbID(id), DeletedAt: catalogTime(at)})
	return affected(n, err)
}

// UpdatePromptTokenCalibration records a successful main-request estimate and provider usage.
func (s *Store) UpdatePromptTokenCalibration(ctx context.Context, id uuid.UUID, estimated, actual int, at time.Time) error {
	if estimated < 1 || actual < 0 {
		return domain.ErrValidation
	}
	n, err := s.queries(ctx).UpdatePromptTokenCalibration(ctx, sqlcgen.UpdatePromptTokenCalibrationParams{SessionID: dbID(id), EstimatedTokens: int64Value(&estimated), ActualTokens: int64Value(&actual), UpdatedAt: catalogTime(at)})
	return affected(n, err)
}

var _ outbound.SessionRepository = (*Store)(nil)
