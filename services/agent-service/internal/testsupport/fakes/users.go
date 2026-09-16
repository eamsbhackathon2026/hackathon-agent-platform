package fakes

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"context"
	"github.com/google/uuid"
	"sort"
	"strings"
	"time"
)

// CreateUser inserts a user with a globally unique email.
func (s *Store) CreateUser(ctx context.Context, v domain.User) error {
	defer s.lock(ctx)()
	if _, ok := s.data.Users[v.ID]; ok {
		return domain.ErrConflict
	}
	for _, u := range s.data.Users {
		if strings.EqualFold(u.Email, v.Email) {
			return domain.ErrConflict
		}
	}
	s.data.Users[v.ID] = cloneUser(v)
	return nil
}

// GetUser returns a user by ID.
func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	defer s.lock(ctx)()
	v, ok := s.data.Users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return cloneUser(v), nil
}

// GetUserForUpdate reads a user protected by the enclosing transaction.
func (s *Store) GetUserForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return s.GetUser(ctx, id)
}

// FindUserByEmail looks up an account for authentication.
func (s *Store) FindUserByEmail(ctx context.Context, email string) (domain.User, error) {
	defer s.lock(ctx)()
	for _, v := range s.data.Users {
		if strings.EqualFold(v.Email, email) {
			return cloneUser(v), nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

// ListUsers lists users by descending creation time and ID.
func (s *Store) ListUsers(ctx context.Context, p domain.PageOptions) ([]domain.User, error) {
	defer s.lock(ctx)()
	result := []domain.User{}
	for _, v := range s.data.Users {
		if before(v.CreatedAt, v.ID, p) {
			result = append(result, cloneUser(v))
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

// UpdateMember updates the supplied member fields in the workspace.
func (s *Store) UpdateMember(ctx context.Context, id uuid.UUID, c domain.MemberChanges) (domain.User, error) {
	defer s.lock(ctx)()
	v, ok := s.data.Users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	if c.Name != nil {
		v.Name = *c.Name
	}
	if c.Role != nil {
		v.Role = *c.Role
	}
	if c.Status != nil {
		v.Status = *c.Status
	}
	s.data.Users[id] = v
	return cloneUser(v), nil
}

// SetPassword replaces the password and clears the temporary password flag.
func (s *Store) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	defer s.lock(ctx)()
	v, ok := s.data.Users[id]
	if !ok {
		return domain.ErrNotFound
	}
	v.PasswordHash = hash
	v.MustChangePassword = false
	s.data.Users[id] = v
	return nil
}

// TouchLastLogin records successful authentication in the workspace.
func (s *Store) TouchLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error {
	defer s.lock(ctx)()
	v, ok := s.data.Users[id]
	if !ok {
		return domain.ErrNotFound
	}
	v.LastLoginAt = &at
	s.data.Users[id] = v
	return nil
}

// CountActiveOwners counts active owners in the workspace.
func (s *Store) CountActiveOwners(ctx context.Context) (int64, error) {
	defer s.lock(ctx)()
	var n int64
	for _, v := range s.data.Users {
		if v.Role == domain.RoleOwner && v.Status == domain.UserActive {
			n++
		}
	}
	return n, nil
}
