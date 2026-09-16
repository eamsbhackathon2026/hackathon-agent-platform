package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
)

// CountUsers counts registered users for first-run setup.
func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	n, err := s.queries(ctx).CountUsers(ctx)
	return n, mapError(err)
}

// LockIdentity serializes first registration and member changes inside a transaction.
func (s *Store) LockIdentity(ctx context.Context) error {
	if _, ok := ctx.Value(txKey{s}).(pgx.Tx); !ok {
		return errors.New("identity lock requires transaction")
	}
	return mapError(s.queries(ctx).LockIdentity(ctx))
}

// CreateUser inserts a user with globally case-insensitive unique email.
func (s *Store) CreateUser(ctx context.Context, v domain.User) error {
	return mapError(s.queries(ctx).CreateUser(ctx, sqlcgen.CreateUserParams{ID: dbID(v.ID), Email: v.Email, PasswordHash: v.PasswordHash, Name: v.Name, Role: string(v.Role), Status: string(v.Status), MustChangePassword: v.MustChangePassword, LastLoginAt: optionalTime(v.LastLoginAt), CreatedAt: dbTime(v.CreatedAt)}))
}

// GetUser loads a user by ID.
func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	v, err := s.queries(ctx).GetUser(ctx, dbID(id))
	return userModel(v), mapError(err)
}

// GetUserForUpdate locks a user row until the current transaction ends.
func (s *Store) GetUserForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	v, err := s.queries(ctx).GetUserForUpdate(ctx, dbID(id))
	return userModel(v), mapError(err)
}

// FindUserByEmail resolves a case-insensitive email for authentication.
func (s *Store) FindUserByEmail(ctx context.Context, email string) (domain.User, error) {
	v, err := s.queries(ctx).FindUserByEmail(ctx, email)
	return userModel(v), mapError(err)
}

// ListUsers returns users in descending creation order.
func (s *Store) ListUsers(ctx context.Context, p domain.PageOptions) ([]domain.User, error) {
	limit, before, id := page(p)
	rows, err := s.queries(ctx).ListUsers(ctx, sqlcgen.ListUsersParams{Limit: limit, BeforeTime: before, BeforeID: id})
	result := make([]domain.User, 0, len(rows))
	for _, v := range rows {
		result = append(result, userModel(v))
	}
	return result, mapError(err)
}

// UpdateMember updates only supplied membership fields.
func (s *Store) UpdateMember(ctx context.Context, id uuid.UUID, c domain.MemberChanges) (domain.User, error) {
	v, err := s.queries(ctx).UpdateMember(ctx, sqlcgen.UpdateMemberParams{ID: dbID(id), Name: textValue(c.Name), Role: textValue(c.Role), Status: textValue(c.Status)})
	return userModel(v), mapError(err)
}

// SetPassword replaces the password hash and clears the forced-change flag.
func (s *Store) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	return affected(s.queries(ctx).SetPassword(ctx, sqlcgen.SetPasswordParams{ID: dbID(id), PasswordHash: hash}))
}

// TouchLastLogin records the successful login time.
func (s *Store) TouchLastLogin(ctx context.Context, id uuid.UUID, t time.Time) error {
	return affected(s.queries(ctx).TouchLastLogin(ctx, sqlcgen.TouchLastLoginParams{ID: dbID(id), LastLoginAt: dbTime(t)}))
}

// CountActiveOwners counts active owners while membership is locked.
func (s *Store) CountActiveOwners(ctx context.Context) (int64, error) {
	n, err := s.queries(ctx).CountActiveOwners(ctx)
	return n, mapError(err)
}
