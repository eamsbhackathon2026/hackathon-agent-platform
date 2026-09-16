package catalog_test

import (
	"context"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

func TestProviderHintNeverRevealsCompleteShortCredential(t *testing.T) {
	for _, tc := range []struct{ name, key, hint string }{
		{"one character", "a", "****"},
		{"four characters", "abcd", "****"},
		{"four unicode characters", "áề🙂界", "****"},
		{"five characters", "abcde", "bcde"},
		{"five unicode characters", "áề🙂界ữ", "ề🙂界ữ"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			created, err := f.s.CreateProvider(ctx, f.p, inbound.ProviderCreateCommand{Name: "Short key", Kind: domain.ProviderGemini, APIKey: tc.key})
			if err != nil {
				t.Fatal(err)
			}
			assertHint := func(p domain.Provider) {
				t.Helper()
				if p.APIKeyHint == nil || *p.APIKeyHint != tc.hint {
					t.Fatalf("unexpected credential hint: %#v", p.APIKeyHint)
				}
				plain, err := (fakes.Cipher{}).Decrypt(p.APIKeyCiphertext, []byte("provider:"+p.ID.String()))
				if err != nil || string(plain) != tc.key {
					t.Fatal("credential changed during masking")
				}
			}
			assertHint(created)
			persisted, err := f.s.GetProvider(ctx, f.p, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertHint(persisted)
			other := f.provider(t, "Existing")
			updated, err := f.s.UpdateProvider(ctx, f.p, other.ID, inbound.ProviderUpdateCommand{APIKey: domain.Change[string]{Set: true, Value: &tc.key}})
			if err != nil {
				t.Fatal(err)
			}
			assertHint(updated)
			omitted, err := f.s.UpdateProvider(ctx, f.p, other.ID, inbound.ProviderUpdateCommand{Name: ptr("Renamed")})
			if err != nil {
				t.Fatal(err)
			}
			assertHint(omitted)
			cleared, err := f.s.UpdateProvider(ctx, f.p, other.ID, inbound.ProviderUpdateCommand{APIKey: domain.Change[string]{Set: true}})
			if err != nil || cleared.APIKeyHint != nil || len(cleared.APIKeyCiphertext) != 0 {
				t.Fatal("null patch did not clear credential and hint", err)
			}
		})
	}
}
