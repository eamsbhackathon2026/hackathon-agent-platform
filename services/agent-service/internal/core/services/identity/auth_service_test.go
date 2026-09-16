package identity_test

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/services/identity"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

var ctx = context.Background()

type fixture struct {
	s     *identity.Service
	db    *fakes.Store
	clock *fakes.Clock
}

func setup(t *testing.T) fixture {
	t.Helper()
	db := fakes.NewStore()
	clock := &fakes.Clock{Time: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}
	s, err := identity.NewService(identity.Dependencies{Users: db, RefreshTokens: db, APIKeys: db, Tx: db, Passwords: fakes.PasswordHasher{}, Tokens: fakes.Tokens{Clock: clock}, KeyHasher: fakes.KeyHasher{}, Cipher: fakes.Cipher{}, Clock: clock, IDs: fakes.IDs{}, Random: fakes.Random{}})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{s, db, clock}
}
func (f fixture) register(t *testing.T, email string) inbound.AuthSession {
	t.Helper()
	v, e := f.s.Register(ctx, inbound.RegisterCommand{Name: "Owner", Email: email, Password: "password12345"})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func (f fixture) principal(t *testing.T, v inbound.AuthSession) domain.Principal {
	t.Helper()
	p, e := f.s.FromAccessToken(ctx, v.AccessToken)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func requireError(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("got %v want %v", err, want)
	}
}

func TestSignupAndLogin(t *testing.T) {
	f := setup(t)
	allowed, err := f.s.SignupAllowed(ctx)
	if err != nil || !allowed {
		t.Fatal(allowed, err)
	}
	registered := f.register(t, " USER@Example.com ")
	if registered.Me.User.Email != "user@example.com" {
		t.Fatal("email not normalized")
	}
	allowed, err = f.s.SignupAllowed(ctx)
	if err != nil || allowed {
		t.Fatal(allowed, err)
	}
	_, err = f.s.Register(ctx, inbound.RegisterCommand{Name: "Owner", Email: "other@example.com", Password: "password12345"})
	requireError(t, err, domain.ErrForbidden)
	logged, err := f.s.Login(ctx, inbound.LoginCommand{Email: "User@Example.com", Password: "password12345", IP: "127.0.0.1"})
	if err != nil || logged.Me.User.LastLoginAt == nil {
		t.Fatal(err)
	}
	p := f.principal(t, logged)
	me, err := f.s.GetMe(ctx, p)
	if err != nil || me.User.ID != registered.Me.User.ID {
		t.Fatal(err)
	}
	for _, email := range []string{"user@example.com", "absent@example.com", "invalid"} {
		_, err = f.s.Login(ctx, inbound.LoginCommand{Email: email, Password: "wrong-password", IP: "bad"})
		requireError(t, err, domain.ErrUnauthenticated)
	}
	for i := 0; i < 10; i++ {
		_, err = f.s.Login(ctx, inbound.LoginCommand{Email: "rate@example.com", Password: "wrong-password", IP: "rate"})
		requireError(t, err, domain.ErrUnauthenticated)
	}
	_, err = f.s.Login(ctx, inbound.LoginCommand{Email: "rate@example.com", Password: "wrong-password", IP: "rate"})
	requireError(t, err, domain.ErrRateLimited)
	f.clock.Time = f.clock.Time.Add(15 * time.Minute)
	_, err = f.s.Login(ctx, inbound.LoginCommand{Email: "rate@example.com", Password: "wrong-password", IP: "rate"})
	requireError(t, err, domain.ErrUnauthenticated)
}
func TestRefreshReuseCommitsAndAbsoluteExpiry(t *testing.T) {
	f := setup(t)
	first := f.register(t, "user@example.com")
	f.clock.Time = f.clock.Time.Add(24 * time.Hour)
	next, err := f.s.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if !next.RefreshExpiresAt.Equal(first.RefreshExpiresAt) {
		t.Fatal("refresh extended absolute expiry")
	}
	_, err = f.s.Refresh(ctx, first.RefreshToken)
	requireError(t, err, domain.ErrUnauthenticated)
	_, err = f.s.Refresh(ctx, next.RefreshToken)
	requireError(t, err, domain.ErrUnauthenticated)
	fresh, err := f.s.Login(ctx, inbound.LoginCommand{Email: "user@example.com", Password: "password12345"})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.Logout(ctx, fresh.RefreshToken); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.Refresh(ctx, fresh.RefreshToken)
	requireError(t, err, domain.ErrUnauthenticated)
	if err = f.s.Logout(ctx, "garbage"); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.Refresh(ctx, "garbage")
	requireError(t, err, domain.ErrUnauthenticated)
}
func TestConcurrentRefreshRevokesSuccessor(t *testing.T) {
	f := setup(t)
	first := f.register(t, "user@example.com")
	var wg sync.WaitGroup
	results := make(chan inbound.AuthSession, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Go(func() { v, e := f.s.Refresh(ctx, first.RefreshToken); results <- v; errs <- e })
	}
	wg.Wait()
	close(results)
	close(errs)
	successes := 0
	for e := range errs {
		if e == nil {
			successes++
		} else {
			requireError(t, e, domain.ErrUnauthenticated)
		}
	}
	if successes != 1 {
		t.Fatal(successes)
	}
	for v := range results {
		if v.RefreshToken != "" {
			_, err := f.s.Refresh(ctx, v.RefreshToken)
			requireError(t, err, domain.ErrUnauthenticated)
		}
	}
}
func TestChangePasswordPreservesCurrentFamily(t *testing.T) {
	f := setup(t)
	first := f.register(t, "user@example.com")
	other, err := f.s.Login(ctx, inbound.LoginCommand{Email: "user@example.com", Password: "password12345"})
	if err != nil {
		t.Fatal(err)
	}
	p := f.principal(t, first)
	requireError(t, f.s.ChangePassword(ctx, p, inbound.ChangePasswordCommand{CurrentPassword: "wrong-password", NewPassword: "new-password123"}), domain.ErrUnauthenticated)
	if err = f.s.ChangePassword(ctx, p, inbound.ChangePasswordCommand{CurrentPassword: "password12345", NewPassword: "new-password123"}); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.Refresh(ctx, other.RefreshToken)
	requireError(t, err, domain.ErrUnauthenticated)
	_, err = f.s.Login(ctx, inbound.LoginCommand{Email: "user@example.com", Password: "password12345"})
	requireError(t, err, domain.ErrUnauthenticated)
	_, err = f.s.Login(ctx, inbound.LoginCommand{Email: "user@example.com", Password: "new-password123"})
	if err != nil {
		t.Fatal(err)
	}
}
