//go:build integration && live

package main

import (
	"bytes"
	"os"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestLiveCatalogGeminiConnection(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}
	server, pool := identityTestServer(t)
	c := newCatalogHTTP(t, server)
	var provider gen.Provider
	c.request("POST", "/v1/providers", map[string]any{"name": "Live Gemini", "kind": "gemini", "api_key": key}, 201, &provider)
	path := "/v1/providers/" + provider.Id.String()
	var encrypted []byte
	if err := pool.QueryRow(t.Context(), "SELECT api_key_ciphertext FROM llm_providers WHERE id=$1", provider.Id).Scan(&encrypted); err != nil || len(encrypted) == 0 || bytes.Contains(encrypted, []byte(key)) {
		t.Fatal("live credential was not encrypted")
	}
	status, raw, err := catalogExchange(server, c.token, "GET", path, nil)
	if err != nil || status != 200 || bytes.Contains(raw, []byte(key)) {
		t.Fatal("live provider read failed or exposed credential")
	}
	var result gen.ProviderTestResult
	c.request("POST", path+"/test", nil, 200, &result)
	if !result.Ok {
		t.Fatal("live provider check failed:", result.Code)
	}
	c.request("GET", path, nil, 200, &provider)
	if provider.Status != "ok" || provider.LastCheckedAt.IsNull() {
		t.Fatal("live provider status not persisted")
	}
	var models gen.ModelPage
	c.request("GET", path+"/models?limit=100", nil, 200, &models)
	if len(models.Items) == 0 {
		t.Fatal("live provider returned no generation models")
	}
	c.request("PATCH", path, map[string]any{"api_key": "invalid-fixture-key"}, 200, &provider)
	c.request("POST", path+"/test", nil, 200, &result)
	if result.Ok || result.Code.MustGet() != "provider_auth_failed" {
		t.Fatal("invalid live credential not mapped to auth failure:", result.Code)
	}
	c.request("GET", path, nil, 200, &provider)
	if provider.Status != "failing" || provider.LastError.MustGet().Code != "provider_auth_failed" {
		t.Fatal("live authentication failure not persisted")
	}
	t.Logf("create/encrypt/test/list and invalid-key failure verified; models=%d", len(models.Items))
}
