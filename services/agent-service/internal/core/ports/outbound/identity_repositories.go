package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// TxManager runs a callback atomically using a transaction-bound context.
type TxManager interface {
	WithinTx(context.Context, func(context.Context) error) error
}

// UserRepository persists accounts and serializes identity invariants.
type UserRepository interface {
	CountUsers(context.Context) (int64, error)
	// LockIdentity serializes first-owner creation and changes to membership.
	// It is a transaction-scoped lock and must be acquired before user row locks.
	LockIdentity(context.Context) error
	CreateUser(context.Context, domain.User) error
	GetUser(context.Context, uuid.UUID) (domain.User, error)
	GetUserForUpdate(context.Context, uuid.UUID) (domain.User, error)
	FindUserByEmail(context.Context, string) (domain.User, error)
	ListUsers(context.Context, domain.PageOptions) ([]domain.User, error)
	UpdateMember(context.Context, uuid.UUID, domain.MemberChanges) (domain.User, error)
	SetPassword(context.Context, uuid.UUID, string) error
	TouchLastLogin(context.Context, uuid.UUID, time.Time) error
	CountActiveOwners(context.Context) (int64, error)
}

// RefreshTokenRepository persists opaque token digests and session-family revocation.
type RefreshTokenRepository interface {
	CreateRefreshToken(context.Context, domain.RefreshToken) error
	// FindRefreshToken is an authentication-only lookup by an unguessable digest.
	FindRefreshToken(context.Context, []byte) (domain.RefreshToken, error)
	GetRefreshTokenForUpdate(context.Context, uuid.UUID) (domain.RefreshToken, error)
	RotateRefreshToken(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	RevokeRefreshFamily(context.Context, uuid.UUID, time.Time) error
	// A nil UUID for exceptFamily revokes every family for the user.
	RevokeUserRefreshTokens(ctx context.Context, userID, exceptFamily uuid.UUID, now time.Time) error
}

// APIKeyRepository persists access keys and their current revocation state.
type APIKeyRepository interface {
	CreateAPIKey(context.Context, domain.APIKey) error
	GetAPIKey(context.Context, uuid.UUID) (domain.APIKey, error)
	// FindAPIKeyByPrefix is an authentication-only lookup; verify the HMAC next.
	FindAPIKeyByPrefix(context.Context, string) (domain.APIKey, error)
	ListAPIKeys(context.Context, domain.PageOptions) ([]domain.APIKey, error)
	RevokeAPIKey(context.Context, uuid.UUID, time.Time) error
	TouchAPIKey(context.Context, uuid.UUID, time.Time) error
}
