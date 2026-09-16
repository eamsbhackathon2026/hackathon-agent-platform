package fakes_test

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
)

func TestCatalogRollbackAndIsolation(t *testing.T) {
	ctx := context.Background()
	s := fakes.NewCatalogStore()
	p := domain.Provider{ID: uuid.New(), Name: "test", APIKeyCiphertext: []byte("encrypted")}
	if err := s.CreateProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	err := s.WithinTx(ctx, func(ctx context.Context) error {
		updated, err := s.GetProviderForUpdate(ctx, p.ID)
		if err != nil {
			return err
		}
		updated.Name = "changed"
		updated.APIKeyCiphertext[0] = 'X'
		if _, err = s.UpdateProvider(ctx, updated); err != nil {
			return err
		}
		return domain.ErrConflict
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	current, err := s.GetProvider(ctx, p.ID)
	if err != nil || current.Name != "test" || string(current.APIKeyCiphertext) != "encrypted" {
		t.Fatal(current, err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic not propagated")
			}
		}()
		_ = s.WithinTx(ctx, func(ctx context.Context) error {
			if err := s.DeleteProvider(ctx, p.ID); err != nil {
				t.Fatal(err)
			}
			panic("rollback")
		})
	}()
	if _, err = s.GetProvider(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	current.APIKeyCiphertext[0] = 'X'
	current, err = s.GetProvider(ctx, p.ID)
	if err != nil || string(current.APIKeyCiphertext) != "encrypted" {
		t.Fatal("mutation escaped snapshot", err)
	}
}
