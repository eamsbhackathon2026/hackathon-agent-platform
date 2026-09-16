package identity_test

import (
	"context"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

type loginTransactionKey struct{}
type markedTransaction struct {
	outbound.TxManager
	calls int
}

func (m *markedTransaction) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	m.calls++
	return m.TxManager.WithinTx(ctx, func(ctx context.Context) error { return fn(context.WithValue(ctx, loginTransactionKey{}, true)) })
}

type inspectedPasswordHasher struct {
	outbound.PasswordHasher
	verify func(context.Context, string, string) (bool, error)
}

func (h inspectedPasswordHasher) Verify(ctx context.Context, password, hash string) (bool, error) {
	return h.verify(ctx, password, hash)
}

func TestLoginVerifiesKnownAndUnknownPasswordsOutsideTransactions(t *testing.T) {
	f := setup(t)
	f.register(t, "owner@example.com")
	tx := &markedTransaction{TxManager: f.db}
	f.s.Tx = tx
	original := f.s.Passwords
	verifications := 0
	f.s.Passwords = inspectedPasswordHasher{PasswordHasher: original, verify: func(ctx context.Context, password, hash string) (bool, error) {
		verifications++
		if ctx.Value(loginTransactionKey{}) != nil {
			t.Error("password hashing held transaction lock")
		}
		return original.Verify(ctx, password, hash)
	}}
	for _, email := range []string{"owner@example.com", "absent@example.com"} {
		_, err := f.s.Login(ctx, inbound.LoginCommand{Email: email, Password: "wrong-password"})
		requireError(t, err, domain.ErrUnauthenticated)
	}
	if verifications != 2 || tx.calls != 0 {
		t.Fatalf("failed logins: verifications=%d transactions=%d", verifications, tx.calls)
	}
	if _, err := f.s.Login(ctx, inbound.LoginCommand{Email: "owner@example.com", Password: "password12345"}); err != nil {
		t.Fatal(err)
	}
	if verifications != 3 || tx.calls != 1 {
		t.Fatalf("successful login: verifications=%d transactions=%d", verifications, tx.calls)
	}
}

func TestLoginRejectsPasswordChangedAfterVerification(t *testing.T) {
	f := setup(t)
	owner := f.register(t, "owner@example.com")
	original := f.s.Passwords
	replacement, err := original.Hash(ctx, "new-password123")
	if err != nil {
		t.Fatal(err)
	}
	f.s.Passwords = inspectedPasswordHasher{PasswordHasher: original, verify: func(ctx context.Context, password, hash string) (bool, error) {
		ok, e := original.Verify(ctx, password, hash)
		if e != nil {
			return false, e
		}
		// Change storage after verifying the snapshot but before Login locks the user.
		if e = f.db.SetPassword(ctx, owner.Me.User.ID, replacement); e != nil {
			return false, e
		}
		return ok, nil
	}}
	_, err = f.s.Login(ctx, inbound.LoginCommand{Email: "owner@example.com", Password: "password12345"})
	requireError(t, err, domain.ErrUnauthenticated)
}
