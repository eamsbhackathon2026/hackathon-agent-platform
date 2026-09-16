package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// Reserve inserts a key or returns the existing committed reservation.
func (s *Store) Reserve(ctx context.Context, value domain.IdempotencyRecord, expiresAt time.Time) (*domain.IdempotencyRecord, error) {
	now := expiresAt.Add(-24 * time.Hour)
	params := sqlcgen.GetIdempotencyKeyParams{PrincipalKind: string(value.PrincipalKind), PrincipalID: dbID(value.PrincipalID), Key: value.Key}
	responseStatus := int32(200)
	if value.ResponseStatus == 202 {
		responseStatus = 202
	}
	if err := s.queries(ctx).DeleteExpiredIdempotencyKey(ctx, sqlcgen.DeleteExpiredIdempotencyKeyParams{PrincipalKind: params.PrincipalKind, PrincipalID: params.PrincipalID, Key: params.Key, ExpiresAt: dbTime(now)}); err != nil {
		return nil, mapError(err)
	}
	_, err := s.queries(ctx).InsertIdempotencyKey(ctx, sqlcgen.InsertIdempotencyKeyParams{PrincipalKind: params.PrincipalKind, PrincipalID: params.PrincipalID, Key: params.Key, RequestHash: value.RequestHash, RunID: dbID(value.RunID), ResponseStatus: responseStatus, CreatedAt: dbTime(now), ExpiresAt: dbTime(expiresAt)})
	if err == nil {
		return nil, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, mapError(err)
	}
	existing, err := s.queries(ctx).GetIdempotencyKey(ctx, params)
	if err != nil {
		return nil, mapError(err)
	}
	model := idempotencyModel(existing)
	return &model, nil
}

// DeleteExpiredIdempotency removes records after their replay window.
func (s *Store) DeleteExpiredIdempotency(ctx context.Context, at time.Time) error {
	return mapError(s.queries(ctx).DeleteExpiredIdempotency(ctx, dbTime(at)))
}

var _ outbound.IdempotencyStore = (*Store)(nil)
