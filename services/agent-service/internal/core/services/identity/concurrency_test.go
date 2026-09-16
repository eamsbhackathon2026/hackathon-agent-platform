package identity_test

import (
	"errors"
	"sync"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

func TestPasswordChangeRacingRefreshRevokesOtherFamily(t *testing.T) {
	for i := 0; i < 10; i++ {
		f := setup(t)
		first := f.register(t, "owner@example.com")
		p := f.principal(t, first)
		other, err := f.s.Login(ctx, inbound.LoginCommand{Email: "owner@example.com", Password: "password12345"})
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var refreshed inbound.AuthSession
		var refreshErr, passwordErr error
		wg.Go(func() { refreshed, refreshErr = f.s.Refresh(ctx, other.RefreshToken) })
		wg.Go(func() {
			passwordErr = f.s.ChangePassword(ctx, p, inbound.ChangePasswordCommand{CurrentPassword: "password12345", NewPassword: "new-password123"})
		})
		wg.Wait()
		if passwordErr != nil {
			t.Fatal(passwordErr)
		}
		if refreshErr != nil && !errors.Is(refreshErr, domain.ErrUnauthenticated) {
			t.Fatal(refreshErr)
		}
		if refreshErr == nil {
			_, err = f.s.Refresh(ctx, refreshed.RefreshToken)
			requireError(t, err, domain.ErrUnauthenticated)
		}
		if _, err = f.s.Refresh(ctx, first.RefreshToken); err != nil {
			t.Fatal("current family lost", err)
		}
	}
}

func TestPasswordChangeRacingLoginRejectsOrRevokesOldPasswordSession(t *testing.T) {
	for i := 0; i < 10; i++ {
		f := setup(t)
		first := f.register(t, "owner@example.com")
		p := f.principal(t, first)
		var wg sync.WaitGroup
		var login inbound.AuthSession
		var loginErr, passwordErr error
		wg.Go(func() {
			login, loginErr = f.s.Login(ctx, inbound.LoginCommand{Email: "owner@example.com", Password: "password12345"})
		})
		wg.Go(func() {
			passwordErr = f.s.ChangePassword(ctx, p, inbound.ChangePasswordCommand{CurrentPassword: "password12345", NewPassword: "new-password123"})
		})
		wg.Wait()
		if passwordErr != nil {
			t.Fatal(passwordErr)
		}
		if loginErr != nil && !errors.Is(loginErr, domain.ErrUnauthenticated) {
			t.Fatal(loginErr)
		}
		if loginErr == nil {
			_, err := f.s.Refresh(ctx, login.RefreshToken)
			requireError(t, err, domain.ErrUnauthenticated)
		}
	}
}
