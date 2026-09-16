//go:build integration

package postgres_test

import (
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/testsupport/pgtest"
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"testing"
	"time"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func missing(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}
func newUser() domain.User {
	return domain.User{ID: uuid.New(), Email: uuid.NewString() + "@example.com", Name: "Owner", PasswordHash: "hash", Role: domain.RoleOwner, Status: domain.UserActive, MustChangePassword: true, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
}
func newKey(user domain.User) domain.APIKey {
	return domain.APIKey{ID: uuid.New(), Name: "Automation", Prefix: uuid.NewString(), KeyHash: []byte("key-hash"), Scopes: []string{"runs:read"}, WebhookSecretCiphertext: []byte("encrypted"), CreatedBy: user.ID, CreatedAt: user.CreatedAt}
}
func newRefresh(user domain.User) domain.RefreshToken {
	return domain.RefreshToken{ID: uuid.New(), UserID: user.ID, FamilyID: uuid.New(), TokenHash: []byte(uuid.NewString()), ExpiresAt: user.CreatedAt.Add(time.Hour), CreatedAt: user.CreatedAt}
}

func TestUserRepository(t *testing.T) {
	ctx := context.Background()
	s := postgres.NewStore(pgtest.NewPool(t))
	u := newUser()
	n, err := s.CountUsers(ctx)
	must(t, err)
	if n != 0 {
		t.Fatal(n)
	}
	must(t, s.CreateUser(ctx, u))
	got, err := s.GetUser(ctx, u.ID)
	must(t, err)
	if got.Email != u.Email || !got.MustChangePassword {
		t.Fatal(got)
	}
	got, err = s.FindUserByEmail(ctx, strings.ToUpper(u.Email))
	must(t, err)
	if got.ID != u.ID {
		t.Fatal(got)
	}
	duplicate := newUser()
	duplicate.Email = strings.ToUpper(u.Email)
	if err = s.CreateUser(ctx, duplicate); !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	n, err = s.CountActiveOwners(ctx)
	must(t, err)
	if n != 1 {
		t.Fatal(n)
	}
	name := "Renamed"
	role := domain.RoleMember
	status := domain.UserDisabled
	got, err = s.UpdateMember(ctx, u.ID, domain.MemberChanges{Name: &name, Role: &role, Status: &status})
	must(t, err)
	if got.Name != name || got.Role != role || got.Status != status {
		t.Fatal(got)
	}
	must(t, s.SetPassword(ctx, u.ID, "new-hash"))
	must(t, s.TouchLastLogin(ctx, u.ID, u.CreatedAt))
	got, err = s.GetUserForUpdate(ctx, u.ID)
	must(t, err)
	if got.PasswordHash != "new-hash" || got.MustChangePassword || got.LastLoginAt == nil {
		t.Fatal(got)
	}
	n, err = s.CountActiveOwners(ctx)
	must(t, err)
	if n != 0 {
		t.Fatal(n)
	}
	newer := newUser()
	newer.CreatedAt = u.CreatedAt.Add(time.Second)
	must(t, s.CreateUser(ctx, newer))
	page, err := s.ListUsers(ctx, domain.PageOptions{Limit: 1})
	must(t, err)
	if len(page) != 1 || page[0].ID != newer.ID {
		t.Fatal(page)
	}
	page, err = s.ListUsers(ctx, domain.PageOptions{Limit: 1, Before: &domain.PageCursor{ID: newer.ID, CreatedAt: newer.CreatedAt}})
	must(t, err)
	if len(page) != 1 || page[0].ID != u.ID {
		t.Fatal(page)
	}
	id := uuid.New()
	_, err = s.GetUser(ctx, id)
	missing(t, err)
	_, err = s.GetUserForUpdate(ctx, id)
	missing(t, err)
	_, err = s.UpdateMember(ctx, id, domain.MemberChanges{})
	missing(t, err)
	missing(t, s.SetPassword(ctx, id, "x"))
	missing(t, s.TouchLastLogin(ctx, id, u.CreatedAt))
	_, err = s.FindUserByEmail(ctx, "absent@example.com")
	missing(t, err)
}

func TestAPIKeyRepository(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewPool(t)
	s := postgres.NewStore(pool)
	u := newUser()
	must(t, s.CreateUser(ctx, u))
	key := newKey(u)
	must(t, s.CreateAPIKey(ctx, key))
	got, err := s.FindAPIKeyByPrefix(ctx, key.Prefix)
	must(t, err)
	if got.ID != key.ID || string(got.WebhookSecretCiphertext) != "encrypted" {
		t.Fatal(got)
	}
	must(t, s.TouchAPIKey(ctx, key.ID, u.CreatedAt))
	var version string
	must(t, pool.QueryRow(ctx, "SELECT xmin::text FROM api_keys WHERE id=$1", key.ID).Scan(&version))
	must(t, s.TouchAPIKey(ctx, key.ID, u.CreatedAt.Add(time.Minute)))
	var after string
	must(t, pool.QueryRow(ctx, "SELECT xmin::text FROM api_keys WHERE id=$1", key.ID).Scan(&after))
	if after != version {
		t.Fatal("throttled touch wrote a row")
	}
	later := u.CreatedAt.Add(61 * time.Second)
	must(t, s.TouchAPIKey(ctx, key.ID, later))
	got, err = s.GetAPIKey(ctx, key.ID)
	must(t, err)
	if got.LastUsedAt == nil || !got.LastUsedAt.Equal(later) {
		t.Fatal(got)
	}
	must(t, s.RevokeAPIKey(ctx, key.ID, later))
	must(t, s.RevokeAPIKey(ctx, key.ID, later.Add(time.Hour)))
	got, err = s.GetAPIKey(ctx, key.ID)
	must(t, err)
	if got.RevokedAt == nil || !got.RevokedAt.Equal(later) {
		t.Fatal("revoke timestamp changed")
	}
	second := newKey(u)
	second.CreatedAt = u.CreatedAt.Add(time.Second)
	must(t, s.CreateAPIKey(ctx, second))
	page, err := s.ListAPIKeys(ctx, domain.PageOptions{Limit: 1})
	must(t, err)
	if len(page) != 1 || page[0].ID != second.ID {
		t.Fatal(page)
	}
	page, err = s.ListAPIKeys(ctx, domain.PageOptions{Limit: 1, Before: &domain.PageCursor{ID: second.ID, CreatedAt: second.CreatedAt}})
	must(t, err)
	if len(page) != 1 || page[0].ID != key.ID {
		t.Fatal(page)
	}
	id := uuid.New()
	_, err = s.GetAPIKey(ctx, id)
	missing(t, err)
	missing(t, s.RevokeAPIKey(ctx, id, later))
	missing(t, s.TouchAPIKey(ctx, id, later))
	_, err = s.FindAPIKeyByPrefix(ctx, "missing")
	missing(t, err)
	invalid := newKey(u)
	invalid.CreatedBy = uuid.New()
	missing(t, s.CreateAPIKey(ctx, invalid))
	invalid = newKey(u)
	invalid.Scopes = []string{"admin"}
	if err = s.CreateAPIKey(ctx, invalid); !errors.Is(err, domain.ErrValidation) {
		t.Fatal(err)
	}
}
