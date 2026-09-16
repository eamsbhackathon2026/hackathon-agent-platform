// Package fakes supplies in-memory dependencies for tests only.
package fakes

import (
	"context"
	"sync"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"github.com/google/uuid"
)

type state struct {
	Users   map[uuid.UUID]domain.User
	Refresh map[uuid.UUID]domain.RefreshToken
	Keys    map[uuid.UUID]domain.APIKey
}

// Store holds isolated identity data behind a transaction mutex.
type Store struct {
	mu   sync.Mutex
	data state
}
type txKey struct{}
type transaction struct {
	store  *Store
	active bool
}

// NewStore creates an empty repository store.
func NewStore() *Store {
	return &Store{data: state{map[uuid.UUID]domain.User{}, map[uuid.UUID]domain.RefreshToken{}, map[uuid.UUID]domain.APIKey{}}}
}
func (s *Store) lock(ctx context.Context) func() {
	if tx, ok := ctx.Value(txKey{}).(*transaction); ok && tx.store == s && tx.active {
		return func() {}
	}
	s.mu.Lock()
	return s.mu.Unlock
}

// WithinTx serializes a callback and rolls back errors or panics.
func (s *Store) WithinTx(ctx context.Context, fn func(context.Context) error) (err error) {
	unlock := s.lock(ctx)
	defer unlock()
	original := s.snapshot()
	tx := &transaction{store: s, active: true}
	defer func() {
		tx.active = false
		if p := recover(); p != nil {
			s.data = original
			panic(p)
		}
		if err != nil {
			s.data = original
		}
	}()
	return fn(context.WithValue(ctx, txKey{}, tx))
}
func (s *Store) snapshot() state {
	d := NewStore().data
	for k, v := range s.data.Users {
		d.Users[k] = cloneUser(v)
	}
	for k, v := range s.data.Refresh {
		d.Refresh[k] = cloneRefresh(v)
	}
	for k, v := range s.data.Keys {
		d.Keys[k] = cloneKey(v)
	}
	return d
}
func copyPointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func cloneUser(v domain.User) domain.User { v.LastLoginAt = copyPointer(v.LastLoginAt); return v }
func cloneRefresh(v domain.RefreshToken) domain.RefreshToken {
	v.TokenHash = append([]byte(nil), v.TokenHash...)
	v.RevokedAt = copyPointer(v.RevokedAt)
	v.ReplacedBy = copyPointer(v.ReplacedBy)
	return v
}
func cloneKey(v domain.APIKey) domain.APIKey {
	v.KeyHash = append([]byte(nil), v.KeyHash...)
	v.Scopes = append([]string(nil), v.Scopes...)
	v.WebhookSecretCiphertext = append([]byte(nil), v.WebhookSecretCiphertext...)
	v.LastUsedAt = copyPointer(v.LastUsedAt)
	v.RevokedAt = copyPointer(v.RevokedAt)
	return v
}
func before(created time.Time, id uuid.UUID, p domain.PageOptions) bool {
	return p.Before == nil || created.Before(p.Before.CreatedAt) || created.Equal(p.Before.CreatedAt) && id.String() < p.Before.ID.String()
}

// CountUsers counts all registered users.
func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	defer s.lock(ctx)()
	return int64(len(s.data.Users)), nil
}

// LockIdentity uses the enclosing transaction lock to serialize identity changes.
func (s *Store) LockIdentity(ctx context.Context) error { defer s.lock(ctx)(); return nil }

var (
	_ outbound.TxManager              = (*Store)(nil)
	_ outbound.UserRepository         = (*Store)(nil)
	_ outbound.RefreshTokenRepository = (*Store)(nil)
	_ outbound.APIKeyRepository       = (*Store)(nil)
)
