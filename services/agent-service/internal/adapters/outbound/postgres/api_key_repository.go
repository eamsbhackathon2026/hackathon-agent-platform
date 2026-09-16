package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
)

// CreateAPIKey persists a hashed key and encrypted webhook secret.
func (s *Store) CreateAPIKey(ctx context.Context, v domain.APIKey) error {
	return mapError(s.queries(ctx).CreateAPIKey(ctx, sqlcgen.CreateAPIKeyParams{ID: dbID(v.ID), Name: v.Name, Prefix: v.Prefix, KeyHash: v.KeyHash, Scopes: v.Scopes, WebhookSecretCiphertext: v.WebhookSecretCiphertext, CreatedBy: dbID(v.CreatedBy), LastUsedAt: optionalTime(v.LastUsedAt), RevokedAt: optionalTime(v.RevokedAt), CreatedAt: dbTime(v.CreatedAt)}))
}

// GetAPIKey loads a key by ID, including revoked keys.
func (s *Store) GetAPIKey(ctx context.Context, id uuid.UUID) (domain.APIKey, error) {
	v, err := s.queries(ctx).GetAPIKey(ctx, dbID(id))
	return apiKeyModel(v), mapError(err)
}

// FindAPIKeyByPrefix resolves an authentication candidate; the caller verifies its HMAC.
func (s *Store) FindAPIKeyByPrefix(ctx context.Context, prefix string) (domain.APIKey, error) {
	v, err := s.queries(ctx).FindAPIKeyByPrefix(ctx, prefix)
	return apiKeyModel(v), mapError(err)
}

// ListAPIKeys returns keys in descending creation order.
func (s *Store) ListAPIKeys(ctx context.Context, p domain.PageOptions) ([]domain.APIKey, error) {
	limit, before, id := page(p)
	rows, err := s.queries(ctx).ListAPIKeys(ctx, sqlcgen.ListAPIKeysParams{Limit: limit, BeforeTime: before, BeforeID: id})
	result := make([]domain.APIKey, 0, len(rows))
	for _, v := range rows {
		result = append(result, apiKeyModel(v))
	}
	return result, mapError(err)
}

// RevokeAPIKey preserves the timestamp of an already revoked key.
func (s *Store) RevokeAPIKey(ctx context.Context, id uuid.UUID, t time.Time) error {
	return affected(s.queries(ctx).RevokeAPIKey(ctx, sqlcgen.RevokeAPIKeyParams{ID: dbID(id), RevokedAt: dbTime(t)}))
}

// TouchAPIKey writes last use only when the previous write is over one minute old.
func (s *Store) TouchAPIKey(ctx context.Context, id uuid.UUID, t time.Time) error {
	n, err := s.queries(ctx).TouchAPIKey(ctx, sqlcgen.TouchAPIKeyParams{ID: dbID(id), LastUsedAt: dbTime(t)})
	if err != nil {
		return mapError(err)
	}
	if n == 0 {
		_, err = s.GetAPIKey(ctx, id)
		return err
	}
	return nil
}
