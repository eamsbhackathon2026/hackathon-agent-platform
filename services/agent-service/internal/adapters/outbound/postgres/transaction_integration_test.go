//go:build integration

package postgres_test

import (
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/testsupport/pgtest"
	"context"
	"errors"
	"testing"
	"time"
)

func TestTransactionRollback(t *testing.T) {
	ctx := context.Background()
	s := postgres.NewStore(pgtest.NewPool(t))
	u := newUser()
	expected := errors.New("abort")
	err := s.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.CreateUser(ctx, u); err != nil {
			return err
		}
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatal(err)
	}
	_, err = s.GetUser(ctx, u.ID)
	missing(t, err)
	func() {
		defer func() {
			if recover() != "test panic" {
				t.Error("panic not propagated")
			}
		}()
		_ = s.WithinTx(ctx, func(ctx context.Context) error { must(t, s.CreateUser(ctx, u)); panic("test panic") })
	}()
	_, err = s.GetUser(ctx, u.ID)
	missing(t, err)
	must(t, s.WithinTx(ctx, func(ctx context.Context) error { return s.CreateUser(ctx, u) }))
}

func TestIdentityLockSerializesTransactions(t *testing.T) {
	ctx := context.Background()
	s := postgres.NewStore(pgtest.NewPool(t))
	if s.LockIdentity(ctx) == nil {
		t.Fatal("lock outside tx accepted")
	}
	held := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- s.WithinTx(ctx, func(ctx context.Context) error {
			if err := s.LockIdentity(ctx); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	select {
	case <-held:
	case err := <-finished:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("first lock timed out")
	}
	blocked, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	err := s.WithinTx(blocked, s.LockIdentity)
	cancel()
	close(release)
	must(t, <-finished)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("concurrent identity lock should time out, got %v", err)
	}
	must(t, s.WithinTx(ctx, s.LockIdentity))
}

func TestSchemaIsolationWithExistingDatabase(t *testing.T) {
	ctx := context.Background()
	first := pgtest.NewPool(t)
	firstStore := postgres.NewStore(first)
	must(t, firstStore.CreateUser(ctx, newUser()))
	t.Setenv("TEST_DATABASE_URL", first.Config().ConnString())
	second := postgres.NewStore(pgtest.NewPool(t))
	n, err := second.CountUsers(ctx)
	must(t, err)
	if n != 0 {
		t.Fatal("second schema contains first schema data")
	}
	must(t, second.CreateUser(ctx, newUser()))
	n, err = firstStore.CountUsers(ctx)
	must(t, err)
	if n != 1 {
		t.Fatal("second schema changed first schema data")
	}
}
