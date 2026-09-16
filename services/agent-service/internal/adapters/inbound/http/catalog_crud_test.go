package http_test

import (
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCatalogProviderSecretPatchAndMergedValidation(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	path := "/v1/providers/" + p.Id.String()
	owner := identityBearer("owner")
	saved, err := f.store.GetProvider(context.Background(), p.Id)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(saved.APIKeyCiphertext, []byte("test-secret-1234")) || len(saved.APIKeyCiphertext) == 0 {
		t.Fatal("secret is not encrypted")
	}
	plain, err := f.cipher.Decrypt(saved.APIKeyCiphertext, []byte("provider:"+p.Id.String()))
	if err != nil || string(plain) != "test-secret-1234" {
		t.Fatal("ciphertext/AAD mismatch")
	}
	f.request(t, "PATCH", path, `{"name":"Renamed"}`, owner, 200)
	kept, _ := f.store.GetProvider(context.Background(), p.Id)
	if !bytes.Equal(kept.APIKeyCiphertext, saved.APIKeyCiphertext) {
		t.Fatal("omitted key changed ciphertext")
	}
	f.request(t, "PATCH", path, `{"base_url":null,"api_key":"must-not-save"}`, owner, 400)
	unchanged, _ := f.store.GetProvider(context.Background(), p.Id)
	if !bytes.Equal(kept.APIKeyCiphertext, unchanged.APIKeyCiphertext) || unchanged.BaseURL == nil {
		t.Fatal("invalid merged patch was persisted")
	}
	f.request(t, "PATCH", path, `{"api_key":"replacement-5678"}`, owner, 200)
	f.request(t, "POST", path+"/test", "", owner, 200)
	calls := f.factory.Connections()
	if len(calls) != 1 || calls[0].APIKey != "replacement-5678" {
		t.Fatal("updated key not used")
	}
	for _, endpoint := range []string{path, "/v1/providers"} {
		r := f.request(t, "GET", endpoint, "", owner, 200)
		if !strings.Contains(r.Body.String(), "5678") {
			t.Fatal("missing safe hint")
		}
		for _, secret := range []string{"test-secret-1234", "replacement-5678", "api_key_ciphertext", `"api_key":`} {
			if strings.Contains(r.Body.String(), secret) {
				t.Fatal("credential leaked")
			}
		}
	}
	a := f.agent(t, p.Id)
	if !a.Ready {
		t.Fatal("configured agent is not ready")
	}
	f.request(t, "PATCH", path, `{"api_key":null}`, owner, 200)
	cleared, _ := f.store.GetProvider(context.Background(), p.Id)
	if len(cleared.APIKeyCiphertext) != 0 || cleared.APIKeyHint != nil {
		t.Fatal("null did not clear credential")
	}
	a = decodeIdentity[gen.Agent](t, f.request(t, "GET", "/v1/agents/"+a.Id.String(), "", owner, 200))
	if a.Ready || a.ReadinessError.IsNull() {
		t.Fatal("cleared credential did not affect readiness")
	}
	if strings.Contains(f.logs.String(), "replacement-5678") {
		t.Fatal("secret leaked to logs")
	}
}

func TestCatalogCreatesGreenNodeWithoutCustomBaseURL(t *testing.T) {
	f := newCatalogFixture(t)
	created := decodeIdentity[gen.Provider](t, f.request(t, "POST", "/v1/providers", `{"name":"GreenNode","kind":"greennode","default_model":"qwen/qwen3.6-flash","api_key":"test-key"}`, identityBearer("owner"), 201))
	if created.Kind != gen.ProviderKindGreennode || !created.BaseUrl.IsNull() || created.DefaultModel.MustGet() != "qwen/qwen3.6-flash" {
		t.Fatalf("unexpected GreenNode provider: %+v", created)
	}
	f.request(t, "POST", "/v1/providers", `{"name":"Custom GreenNode","kind":"greennode","base_url":"https://example.com/v1","api_key":"test-key"}`, identityBearer("owner"), 400)
}

func TestCatalogAgentDefaultsArchiveAndProviderConflict(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	a := f.agent(t, p.Id)
	owner := identityBearer("owner")
	path := "/v1/agents/" + a.Id.String()
	if a.MaxIterations != 8 || a.TimeoutSeconds != 120 || !a.Temperature.IsNull() || !a.MaxOutputTokens.IsNull() {
		t.Fatal("wrong agent defaults")
	}
	a = decodeIdentity[gen.Agent](t, f.request(t, "PATCH", path, `{"temperature":0,"max_output_tokens":42,"description":"Updated"}`, owner, 200))
	temp, err := a.Temperature.Get()
	if err != nil || temp != 0 || a.Description != "Updated" {
		t.Fatal("zero temperature or patch lost")
	}
	a = decodeIdentity[gen.Agent](t, f.request(t, "PATCH", path, `{"temperature":null,"max_output_tokens":null}`, owner, 200))
	if !a.Temperature.IsNull() || !a.MaxOutputTokens.IsNull() {
		t.Fatal("nullable settings not cleared")
	}
	conflict := f.request(t, "DELETE", "/v1/providers/"+p.Id.String(), "", owner, 409)
	if !strings.Contains(conflict.Body.String(), "related_agents") || !strings.Contains(conflict.Body.String(), a.Id.String()) {
		t.Fatal("conflict lacks related assistant")
	}
	page := decodeIdentity[gen.AgentPage](t, f.request(t, "GET", "/v1/agents", "", owner, 200))
	if len(page.Items) != 1 {
		t.Fatal("agent missing from list")
	}
	f.request(t, "DELETE", path, "", owner, 204)
	f.request(t, "GET", path, "", owner, 404)
	page = decodeIdentity[gen.AgentPage](t, f.request(t, "GET", "/v1/agents", "", owner, 200))
	if len(page.Items) != 0 {
		t.Fatal("archived agent listed")
	}
	reused := f.agent(t, p.Id)
	if reused.Id == a.Id {
		t.Fatal("name reuse restored archived record")
	}
	f.request(t, "DELETE", "/v1/agents/"+reused.Id.String(), "", owner, 204)
	f.request(t, "DELETE", "/v1/providers/"+p.Id.String(), "", owner, 204)
	f.request(t, "GET", "/v1/providers/"+p.Id.String(), "", owner, 404)
}
