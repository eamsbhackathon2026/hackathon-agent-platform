package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
)

// CreateRefreshToken inserts a token before any predecessor links to it.
func (s *Store) CreateRefreshToken(ctx context.Context, v domain.RefreshToken) error {
	return mapError(s.queries(ctx).CreateRefreshToken(ctx, sqlcgen.CreateRefreshTokenParams{ID: dbID(v.ID), UserID: dbID(v.UserID), FamilyID: dbID(v.FamilyID), TokenHash: v.TokenHash, ExpiresAt: dbTime(v.ExpiresAt), RevokedAt: optionalTime(v.RevokedAt), ReplacedBy: optionalID(v.ReplacedBy), CreatedAt: dbTime(v.CreatedAt)}))
}

// FindRefreshToken resolves an opaque token digest for authentication.
func (s *Store) FindRefreshToken(ctx context.Context, hash []byte) (domain.RefreshToken, error) {
	v, err := s.queries(ctx).FindRefreshToken(ctx, hash)
	return refreshModel(v), mapError(err)
}

// GetRefreshTokenForUpdate locks a token row until the current transaction ends.
func (s *Store) GetRefreshTokenForUpdate(ctx context.Context, id uuid.UUID) (domain.RefreshToken, error) {
	v, err := s.queries(ctx).GetRefreshTokenForUpdate(ctx, dbID(id))
	return refreshModel(v), mapError(err)
}

// RotateRefreshToken links an active token to an existing successor in its family.
func (s *Store) RotateRefreshToken(ctx context.Context, id, replaced uuid.UUID, t time.Time) error {
	return affected(s.queries(ctx).RotateRefreshToken(ctx, sqlcgen.RotateRefreshTokenParams{ID: dbID(id), ReplacedBy: dbID(replaced), RevokedAt: dbTime(t)}))
}

// RevokeRefreshFamily revokes a family without changing prior revocation timestamps.
func (s *Store) RevokeRefreshFamily(ctx context.Context, family uuid.UUID, t time.Time) error {
	return affected(s.queries(ctx).RevokeRefreshFamily(ctx, sqlcgen.RevokeRefreshFamilyParams{FamilyID: dbID(family), RevokedAt: dbTime(t)}))
}

// RevokeUserRefreshTokens revokes user sessions, optionally preserving one family.
func (s *Store) RevokeUserRefreshTokens(ctx context.Context, user, except uuid.UUID, t time.Time) error {
	if _, err := s.GetUser(ctx, user); err != nil {
		return err
	}
	return mapError(s.queries(ctx).RevokeUserRefreshTokens(ctx, sqlcgen.RevokeUserRefreshTokensParams{UserID: dbID(user), ExceptFamily: dbID(except), RevokedAt: dbTime(t)}))
}
