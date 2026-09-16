package fakes

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"bytes"
	"context"
	"github.com/google/uuid"
	"sort"
	"time"
)

// CreateRefreshToken stores a refresh token with a unique digest.
func (s *Store) CreateRefreshToken(ctx context.Context, v domain.RefreshToken) error {
	defer s.lock(ctx)()
	if _, ok := s.data.Refresh[v.ID]; ok {
		return domain.ErrConflict
	}
	for _, r := range s.data.Refresh {
		if bytes.Equal(r.TokenHash, v.TokenHash) {
			return domain.ErrConflict
		}
	}
	s.data.Refresh[v.ID] = cloneRefresh(v)
	return nil
}

// FindRefreshToken looks up a refresh token by digest for authentication.
func (s *Store) FindRefreshToken(ctx context.Context, hash []byte) (domain.RefreshToken, error) {
	defer s.lock(ctx)()
	for _, v := range s.data.Refresh {
		if bytes.Equal(v.TokenHash, hash) {
			return cloneRefresh(v), nil
		}
	}
	return domain.RefreshToken{}, domain.ErrNotFound
}

// GetRefreshTokenForUpdate reads a token under the enclosing transaction.
func (s *Store) GetRefreshTokenForUpdate(ctx context.Context, id uuid.UUID) (domain.RefreshToken, error) {
	defer s.lock(ctx)()
	v, ok := s.data.Refresh[id]
	if !ok {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	return cloneRefresh(v), nil
}

// RotateRefreshToken revokes a token and records its replacement.
func (s *Store) RotateRefreshToken(ctx context.Context, id, next uuid.UUID, at time.Time) error {
	defer s.lock(ctx)()
	v, ok := s.data.Refresh[id]
	if !ok {
		return domain.ErrNotFound
	}
	if v.RevokedAt != nil {
		return domain.ErrNotFound
	}
	v.RevokedAt = &at
	v.ReplacedBy = &next
	s.data.Refresh[id] = v
	return nil
}

// RevokeRefreshFamily revokes all active tokens in a family.
func (s *Store) RevokeRefreshFamily(ctx context.Context, family uuid.UUID, at time.Time) error {
	defer s.lock(ctx)()
	for id, v := range s.data.Refresh {
		if v.FamilyID == family && v.RevokedAt == nil {
			v.RevokedAt = copyPointer(&at)
			s.data.Refresh[id] = v
		}
	}
	return nil
}

// RevokeUserRefreshTokens revokes user tokens except the supplied family.
func (s *Store) RevokeUserRefreshTokens(ctx context.Context, user, except uuid.UUID, at time.Time) error {
	defer s.lock(ctx)()
	for id, v := range s.data.Refresh {
		if v.UserID == user && (except == uuid.Nil || v.FamilyID != except) && v.RevokedAt == nil {
			v.RevokedAt = copyPointer(&at)
			s.data.Refresh[id] = v
		}
	}
	return nil
}

// CreateAPIKey stores an API key with a unique prefix.
func (s *Store) CreateAPIKey(ctx context.Context, v domain.APIKey) error {
	defer s.lock(ctx)()
	if _, ok := s.data.Keys[v.ID]; ok {
		return domain.ErrConflict
	}
	for _, k := range s.data.Keys {
		if k.Prefix == v.Prefix {
			return domain.ErrConflict
		}
	}
	s.data.Keys[v.ID] = cloneKey(v)
	return nil
}

// GetAPIKey returns an API key by ID.
func (s *Store) GetAPIKey(ctx context.Context, id uuid.UUID) (domain.APIKey, error) {
	defer s.lock(ctx)()
	v, ok := s.data.Keys[id]
	if !ok {
		return domain.APIKey{}, domain.ErrNotFound
	}
	return cloneKey(v), nil
}

// FindAPIKeyByPrefix looks up an API key for subsequent hash verification.
func (s *Store) FindAPIKeyByPrefix(ctx context.Context, prefix string) (domain.APIKey, error) {
	defer s.lock(ctx)()
	for _, v := range s.data.Keys {
		if v.Prefix == prefix {
			return cloneKey(v), nil
		}
	}
	return domain.APIKey{}, domain.ErrNotFound
}

// ListAPIKeys lists keys by descending creation time and ID.
func (s *Store) ListAPIKeys(ctx context.Context, p domain.PageOptions) ([]domain.APIKey, error) {
	defer s.lock(ctx)()
	result := []domain.APIKey{}
	for _, v := range s.data.Keys {
		if before(v.CreatedAt, v.ID, p) {
			result = append(result, cloneKey(v))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID.String() > result[j].ID.String()
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if p.Limit > 0 && len(result) > p.Limit {
		result = result[:p.Limit]
	}
	return result, nil
}

// RevokeAPIKey revokes a key by ID.
func (s *Store) RevokeAPIKey(ctx context.Context, id uuid.UUID, at time.Time) error {
	defer s.lock(ctx)()
	v, ok := s.data.Keys[id]
	if !ok {
		return domain.ErrNotFound
	}
	if v.RevokedAt == nil {
		v.RevokedAt = &at
		s.data.Keys[id] = v
	}
	return nil
}

// TouchAPIKey updates usage at most once per minute.
func (s *Store) TouchAPIKey(ctx context.Context, id uuid.UUID, at time.Time) error {
	defer s.lock(ctx)()
	v, ok := s.data.Keys[id]
	if !ok {
		return domain.ErrNotFound
	}
	if v.LastUsedAt == nil || at.Sub(*v.LastUsedAt) >= time.Minute {
		v.LastUsedAt = &at
		s.data.Keys[id] = v
	}
	return nil
}
