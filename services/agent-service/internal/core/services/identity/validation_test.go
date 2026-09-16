package identity_test

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/core/services/identity"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestInvalidServiceInputs(t *testing.T) {
	if _, err := identity.NewService(identity.Dependencies{}); err == nil {
		t.Fatal("missing dependencies accepted")
	}
	f := setup(t)
	for _, c := range []inbound.RegisterCommand{{Name: "Name", Email: "bad", Password: "password12345"}, {Name: "", Email: "owner@example.com", Password: "password12345"}, {Name: "Name", Email: "owner@example.com", Password: "short"}} {
		_, err := f.s.Register(ctx, c)
		requireError(t, err, domain.ErrValidation)
	}
	owner := f.register(t, "owner@example.com")
	p := f.principal(t, owner)
	for _, c := range []inbound.MemberCreateCommand{{Name: "Name", Email: "user@example.com", Role: "bad"}, {Name: "Name", Email: "bad"}, {Name: "", Email: "user@example.com"}} {
		_, err := f.s.CreateMember(ctx, p, c)
		requireError(t, err, domain.ErrValidation)
	}
	badName := ""
	badRole := domain.Role("bad")
	badStatus := domain.UserStatus("bad")
	for _, c := range []domain.MemberChanges{{Name: &badName}, {Role: &badRole}, {Status: &badStatus}} {
		_, err := f.s.UpdateMember(ctx, p, p.UserID, c)
		requireError(t, err, domain.ErrValidation)
	}
	_, err := f.s.UpdateMember(ctx, domain.Principal{}, p.UserID, domain.MemberChanges{})
	requireError(t, err, domain.ErrForbidden)
	for _, page := range []inbound.PageRequest{{Limit: -1}, {Limit: 101}, {Cursor: "!"}, {Cursor: base64.RawURLEncoding.EncodeToString([]byte(`{}`))}} {
		_, err = f.s.ListMembers(ctx, p, page)
		requireError(t, err, domain.ErrValidation)
		_, err = f.s.ListAPIKeys(ctx, p, page)
		requireError(t, err, domain.ErrValidation)
	}
	for _, c := range []inbound.APIKeyCreateCommand{{Name: ""}, {Name: "Key"}, {Name: "Key", Scopes: []string{"admin"}}} {
		_, err = f.s.CreateAPIKey(ctx, p, c)
		requireError(t, err, domain.ErrValidation)
	}
	_, err = f.s.GetMe(ctx, domain.Principal{})
	requireError(t, err, domain.ErrForbidden)
	_, err = f.s.FromAccessToken(ctx, "invalid")
	requireError(t, err, domain.ErrUnauthenticated)
	requireError(t, f.s.ChangePassword(ctx, domain.Principal{}, inbound.ChangePasswordCommand{}), domain.ErrForbidden)
	requireError(t, f.s.ChangePassword(ctx, p, inbound.ChangePasswordCommand{NewPassword: "short"}), domain.ErrValidation)
	requireError(t, f.s.ChangePassword(ctx, p, inbound.ChangePasswordCommand{CurrentPassword: "short", NewPassword: "password12345"}), domain.ErrUnauthenticated)
	missing := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	_, err = f.s.Refresh(ctx, missing)
	requireError(t, err, domain.ErrUnauthenticated)
	if err = f.s.Logout(ctx, missing); err != nil {
		t.Fatal(err)
	}
	f.clock.Time = f.clock.Time.Add(31 * 24 * time.Hour)
	_, err = f.s.Refresh(ctx, owner.RefreshToken)
	requireError(t, err, domain.ErrUnauthenticated)
}

type failingIssuer struct{ outbound.AccessTokenIssuer }

func (failingIssuer) Issue(uuid.UUID, uuid.UUID) (string, time.Time, error) {
	return "", time.Time{}, errors.New("issuer unavailable")
}
func TestSessionCreationFailureRollsBack(t *testing.T) {
	f := setup(t)
	issuer := f.s.Tokens
	f.s.Tokens = failingIssuer{issuer}
	_, err := f.s.Register(ctx, inbound.RegisterCommand{Name: "Owner", Email: "owner@example.com", Password: "password12345"})
	if err == nil {
		t.Fatal("failure missing")
	}
	count, err := f.db.CountUsers(ctx)
	if err != nil || count != 0 {
		t.Fatal("registration not rolled back", count, err)
	}
	f.s.Tokens = issuer
	first := f.register(t, "owner@example.com")
	f.s.Tokens = failingIssuer{issuer}
	_, err = f.s.Refresh(ctx, first.RefreshToken)
	if err == nil {
		t.Fatal("failure missing")
	}
	f.s.Tokens = issuer
	if _, err = f.s.Refresh(ctx, first.RefreshToken); err != nil {
		t.Fatal("failed refresh consumed old token", err)
	}
}
