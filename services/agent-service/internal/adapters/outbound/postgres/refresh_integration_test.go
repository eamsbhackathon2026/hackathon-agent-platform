//go:build integration

package postgres_test

import (
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/testsupport/pgtest"
	"context"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestRefreshTokenRepository(t *testing.T) {
	ctx := context.Background()
	s := postgres.NewStore(pgtest.NewPool(t))
	u := newUser()
	must(t, s.CreateUser(ctx, u))
	original := newRefresh(u)
	must(t, s.CreateRefreshToken(ctx, original))
	got, err := s.FindRefreshToken(ctx, original.TokenHash)
	must(t, err)
	if got.ID != original.ID {
		t.Fatal(got)
	}
	successor := newRefresh(u)
	successor.FamilyID = original.FamilyID
	must(t, s.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.GetRefreshTokenForUpdate(ctx, original.ID); err != nil {
			return err
		}
		if err := s.CreateRefreshToken(ctx, successor); err != nil {
			return err
		}
		return s.RotateRefreshToken(ctx, original.ID, successor.ID, u.CreatedAt)
	}))
	got, err = s.FindRefreshToken(ctx, original.TokenHash)
	must(t, err)
	if got.ReplacedBy == nil || *got.ReplacedBy != successor.ID || got.RevokedAt == nil {
		t.Fatal(got)
	}
	missing(t, s.RotateRefreshToken(ctx, original.ID, successor.ID, u.CreatedAt))
	must(t, s.RevokeRefreshFamily(ctx, original.FamilyID, u.CreatedAt))
	must(t, s.RevokeRefreshFamily(ctx, original.FamilyID, u.CreatedAt.Add(time.Hour)))
	got, err = s.FindRefreshToken(ctx, successor.TokenHash)
	must(t, err)
	if got.RevokedAt == nil || !got.RevokedAt.Equal(u.CreatedAt) {
		t.Fatal(got)
	}
	preserved := newRefresh(u)
	revoked := newRefresh(u)
	must(t, s.CreateRefreshToken(ctx, preserved))
	must(t, s.CreateRefreshToken(ctx, revoked))
	must(t, s.RevokeUserRefreshTokens(ctx, u.ID, preserved.FamilyID, u.CreatedAt))
	got, err = s.FindRefreshToken(ctx, preserved.TokenHash)
	must(t, err)
	if got.RevokedAt != nil {
		t.Fatal("except family revoked")
	}
	got, err = s.FindRefreshToken(ctx, revoked.TokenHash)
	must(t, err)
	if got.RevokedAt == nil {
		t.Fatal("other family not revoked")
	}
	must(t, s.RevokeUserRefreshTokens(ctx, u.ID, uuid.Nil, u.CreatedAt))
	got, err = s.FindRefreshToken(ctx, preserved.TokenHash)
	must(t, err)
	if got.RevokedAt == nil {
		t.Fatal("full revoke missed family")
	}
	id := uuid.New()
	_, err = s.GetRefreshTokenForUpdate(ctx, id)
	missing(t, err)
	_, err = s.FindRefreshToken(ctx, []byte("missing"))
	missing(t, err)
	missing(t, s.RotateRefreshToken(ctx, id, successor.ID, u.CreatedAt))
	missing(t, s.RevokeRefreshFamily(ctx, id, u.CreatedAt))
	missing(t, s.RevokeUserRefreshTokens(ctx, id, uuid.Nil, u.CreatedAt))
	invalid := newRefresh(u)
	invalid.UserID = uuid.New()
	missing(t, s.CreateRefreshToken(ctx, invalid))
}

func TestRefreshReplacementIntegrity(t *testing.T) {
	ctx := context.Background()
	s := postgres.NewStore(pgtest.NewPool(t))
	u := newUser()
	other := newUser()
	must(t, s.CreateUser(ctx, u))
	must(t, s.CreateUser(ctx, other))
	original := newRefresh(u)
	must(t, s.CreateRefreshToken(ctx, original))
	foreignUser := newRefresh(other)
	foreignUser.FamilyID = original.FamilyID
	foreignFamily := newRefresh(u)
	for _, successor := range []domain.RefreshToken{foreignUser, foreignFamily} {
		must(t, s.CreateRefreshToken(ctx, successor))
		missing(t, s.RotateRefreshToken(ctx, original.ID, successor.ID, u.CreatedAt))
	}
	missing(t, s.RotateRefreshToken(ctx, original.ID, uuid.New(), u.CreatedAt))
	got, err := s.FindRefreshToken(ctx, original.TokenHash)
	must(t, err)
	if got.RevokedAt != nil || got.ReplacedBy != nil {
		t.Fatal("failed rotation mutated original")
	}
}
