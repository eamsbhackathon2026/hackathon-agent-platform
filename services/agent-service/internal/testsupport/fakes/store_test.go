package fakes

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
	"testing"
	"time"
)

func TestRollbackAndUserLookup(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	user := uuid.New()
	other := uuid.New()
	if err := s.CreateUser(ctx, domain.User{ID: user, Email: "owner@example.com", Role: domain.RoleOwner, Status: domain.UserActive}); err != nil {
		t.Fatal(err)
	}
	abort := errors.New("abort")
	err := s.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.SetPassword(ctx, user, "changed"); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("rollback error: %v", err)
	}
	got, err := s.GetUser(ctx, user)
	if err != nil || got.PasswordHash != "" {
		t.Fatalf("rollback failed: %v, %v", got, err)
	}
	if _, err := s.GetUser(ctx, other); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown user read: %v", err)
	}
	if err := s.SetPassword(ctx, other, "bad"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown user update: %v", err)
	}
	key := domain.APIKey{ID: uuid.New(), Prefix: "prefix", KeyHash: []byte{1}, Scopes: []string{"runs:read"}}
	if err := s.CreateAPIKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	key.KeyHash[0] = 2
	loaded, err := s.GetAPIKey(ctx, key.ID)
	if err != nil || loaded.KeyHash[0] != 1 {
		t.Fatal("input was not copied")
	}
	loaded.Scopes[0] = "bad"
	loaded, err = s.GetAPIKey(ctx, key.ID)
	if err != nil || loaded.Scopes[0] != "runs:read" {
		t.Fatal("output was not copied")
	}
}
func TestConcurrentTransactionsSerialize(t *testing.T) {
	s := NewStore()
	ctx := context.Background()
	var wg sync.WaitGroup
	var count int
	for range 20 {
		wg.Go(func() {
			err := s.WithinTx(ctx, func(ctx context.Context) error {
				if err := s.LockIdentity(ctx); err != nil {
					return err
				}
				count++
				return s.CreateUser(ctx, domain.User{ID: uuid.New(), Email: uuid.NewString() + "@example.com"})
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	users, err := s.CountUsers(ctx)
	if err != nil || users != 20 || count != 20 {
		t.Fatalf("count %d users %d error %v", count, users, err)
	}
}
func TestSecurityDoubles(t *testing.T) {
	ctx := context.Background()
	p := PasswordHasher{}
	hash, err := p.Hash(ctx, "password")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := p.Verify(ctx, "password", hash); err != nil || !ok {
		t.Fatal("password verification failed")
	}
	c := Cipher{}
	encrypted, err := c.Encrypt([]byte("secret"), []byte("record"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Decrypt(encrypted, []byte("other")); err == nil {
		t.Fatal("wrong AAD accepted")
	}
	clock := &Clock{Time: time.Now()}
	tokens := Tokens{Clock: clock}
	encoded, expires, err := tokens.Issue(uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Verify(encoded); err != nil {
		t.Fatal(err)
	}
	clock.Time = expires
	if _, err := tokens.Verify(encoded); err == nil {
		t.Fatal("expired token accepted")
	}
}
