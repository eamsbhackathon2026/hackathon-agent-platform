package identity_test

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"strings"
	"testing"
	"time"
)

func TestAPIKeyLifecycleAndOwnership(t *testing.T) {
	f := setup(t)
	owner := f.register(t, "owner@example.com")
	p := f.principal(t, owner)
	created, err := f.s.CreateAPIKey(ctx, p, inbound.APIKeyCreateCommand{Name: "Integration", Scopes: []string{"runs:read", "runs:write", "runs:read"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Key) != 56 || !strings.HasPrefix(created.WebhookSecret, "whsec_") || len(created.APIKey.Scopes) != 2 || created.APIKey.KeyHash != nil || created.APIKey.WebhookSecretCiphertext != nil {
		t.Fatal("wrong key material")
	}
	stored, err := f.db.GetAPIKey(ctx, created.APIKey.ID)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := (fakes.Cipher{}).Decrypt(stored.WebhookSecretCiphertext, []byte("webhook:"+stored.ID.String()))
	if err != nil || string(plain) != created.WebhookSecret {
		t.Fatal("AAD or stored secret", err)
	}
	kp, err := f.s.FromAPIKey(ctx, created.Key)
	if err != nil || kp.APIKeyID != stored.ID {
		t.Fatal(err)
	}
	requireError(t, domain.Authorize(kp, domain.ActionRunsRead, &p.UserID), domain.ErrForbidden)
	if err = domain.Authorize(kp, domain.ActionRunsRead, &kp.APIKeyID); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.ListAPIKeys(ctx, kp, inbound.PageRequest{})
	requireError(t, err, domain.ErrForbidden)
	_, err = f.s.CreateAPIKey(ctx, kp, inbound.APIKeyCreateCommand{})
	requireError(t, err, domain.ErrForbidden)
	requireError(t, f.s.RevokeAPIKey(ctx, kp, stored.ID), domain.ErrForbidden)
	firstTouch, _ := f.db.GetAPIKey(ctx, stored.ID)
	f.clock.Time = f.clock.Time.Add(30 * time.Second)
	_, err = f.s.FromAPIKey(ctx, created.Key)
	if err != nil {
		t.Fatal(err)
	}
	same, _ := f.db.GetAPIKey(ctx, stored.ID)
	if !same.LastUsedAt.Equal(*firstTouch.LastUsedAt) {
		t.Fatal("touched too soon")
	}
	f.clock.Time = f.clock.Time.Add(time.Minute)
	_, err = f.s.FromAPIKey(ctx, created.Key)
	if err != nil {
		t.Fatal(err)
	}
	later, _ := f.db.GetAPIKey(ctx, stored.ID)
	if !later.LastUsedAt.After(*same.LastUsedAt) {
		t.Fatal("not touched")
	}
	_, err = f.s.CreateAPIKey(ctx, p, inbound.APIKeyCreateCommand{Name: "Second", Scopes: []string{"runs:write"}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := f.s.ListAPIKeys(ctx, p, inbound.PageRequest{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatal(page, err)
	}
	next, err := f.s.ListAPIKeys(ctx, p, inbound.PageRequest{Limit: 1, Cursor: *page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.NextCursor != nil {
		t.Fatal(next, err)
	}
	for _, item := range append(page.Items, next.Items...) {
		if item.KeyHash != nil || item.WebhookSecretCiphertext != nil {
			t.Fatal("list leaked secrets")
		}
	}
	if err = f.s.RevokeAPIKey(ctx, p, stored.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.FromAPIKey(ctx, created.Key)
	requireError(t, err, domain.ErrUnauthenticated)
	for _, key := range []string{"bad", "apk_" + strings.Repeat("x", 8) + "_" + strings.Repeat("x", 43), created.Key[:55] + "!"} {
		_, err = f.s.FromAPIKey(ctx, key)
		requireError(t, err, domain.ErrUnauthenticated)
	}
}
