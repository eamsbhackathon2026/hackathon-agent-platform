//go:build integration

package main

import (
	"bytes"
	"net/url"
	"sync"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/adapters/outbound/security"
)

func TestCatalogWiringProviderProtocols(t *testing.T) {
	server, pool := identityTestServer(t)
	c := newCatalogHTTP(t, server)
	for _, kind := range []string{"openai_compatible", "gemini"} {
		t.Run(kind, func(t *testing.T) {
			c := c
			c.t = t
			upstream := newCatalogUpstream(t, kind)
			p := c.provider(kind, upstream.server.URL, kind)
			path := "/v1/providers/" + p.Id.String()
			var encrypted []byte
			if err := pool.QueryRow(t.Context(), "SELECT api_key_ciphertext FROM llm_providers WHERE id=$1", p.Id).Scan(&encrypted); err != nil {
				t.Fatal("read encrypted credential")
			}
			if len(encrypted) == 0 || bytes.Contains(encrypted, []byte(catalogGoodKey)) {
				t.Fatal("credential was not encrypted")
			}
			cipher, err := security.NewAESGCMSecretCipher(bytes.Repeat([]byte{42}, 32))
			if err != nil {
				t.Fatal("initialize test cipher")
			}
			plain, err := cipher.Decrypt(encrypted, []byte("provider:"+p.Id.String()))
			if err != nil || string(plain) != catalogGoodKey {
				t.Fatal("credential AAD round trip failed")
			}
			if _, err = cipher.Decrypt(encrypted, []byte("provider:wrong-id")); err == nil {
				t.Fatal("wrong AAD accepted")
			}
			c.request("PATCH", path, map[string]any{"name": kind + " renamed"}, 200, &p)
			var result gen.ProviderTestResult
			c.request("POST", path+"/test", nil, 200, &result)
			if !result.Ok || !result.Code.IsNull() {
				t.Fatal("provider check failed")
			}
			c.request("GET", path, nil, 200, &p)
			if p.Status != "ok" || p.LastCheckedAt.IsNull() {
				t.Fatal("successful check not persisted")
			}
			var models gen.ModelPage
			c.request("GET", path+"/models?limit=1", nil, 200, &models)
			if len(models.Items) != 1 || models.Items[0].Id != "alpha" || models.NextCursor.IsNull() {
				t.Fatal("models not sorted and paginated")
			}
			c.request("GET", path+"/models?limit=1&cursor="+url.QueryEscape(models.NextCursor.MustGet()), nil, 200, &models)
			if len(models.Items) != 1 || models.Items[0].Id != "zeta" || !models.NextCursor.IsNull() {
				t.Fatal("models not deduplicated")
			}
			c.request("PATCH", path, map[string]any{"api_key": catalogBadKey}, 200, &p)
			c.request("POST", path+"/test", nil, 200, &result)
			if result.Ok || result.Code.MustGet() != "provider_auth_failed" {
				t.Fatal("bad key not mapped to safe auth failure")
			}
			c.request("GET", path, nil, 200, &p)
			if p.Status != "failing" || p.LastError.MustGet().Code != "provider_auth_failed" {
				t.Fatal("failure not persisted")
			}
			c.request("GET", path+"/models", nil, 502, nil)
			c.request("PATCH", path, map[string]any{"api_key": nil}, 200, &p)
			if !p.ApiKeyHint.IsNull() {
				t.Fatal("cleared credential hint remains")
			}
			c.request("POST", path+"/test", nil, 200, &result)
			if result.Ok || result.Code.MustGet() != "provider_not_configured" {
				t.Fatal("missing key not actionable")
			}
		})
	}
}

func TestCatalogWiringAgentLifecycle(t *testing.T) {
	server, pool := identityTestServer(t)
	c := newCatalogHTTP(t, server)
	upstream := newCatalogUpstream(t, "openai_compatible")
	p := c.provider("openai_compatible", upstream.server.URL, "Connection")
	providerPath := "/v1/providers/" + p.Id.String()
	body := map[string]any{"name": "Assistant", "provider_id": p.Id, "model": "alpha"}
	var a gen.Agent
	c.request("POST", "/v1/agents", body, 201, &a)
	path := "/v1/agents/" + a.Id.String()
	if a.MaxIterations != 8 || a.TimeoutSeconds != 120 || !a.Ready || !a.Temperature.IsNull() {
		t.Fatal("incorrect agent defaults")
	}
	c.request("PATCH", path, map[string]any{"temperature": 0.75, "max_output_tokens": 64}, 200, &a)
	c.request("PATCH", path, map[string]any{"description": "Updated"}, 200, &a)
	if a.Temperature.MustGet() != 0.75 || a.MaxOutputTokens.MustGet() != 64 {
		t.Fatal("omitted nullable settings changed")
	}
	c.request("PATCH", path, map[string]any{"temperature": nil, "max_output_tokens": nil}, 200, &a)
	if !a.Temperature.IsNull() || !a.MaxOutputTokens.IsNull() {
		t.Fatal("nullable settings not cleared")
	}
	member := c.member()
	member.request("GET", path, nil, 200, nil)
	member.request("GET", providerPath, nil, 200, nil)
	member.request("POST", "/v1/agents", body, 403, nil)
	var problem gen.Problem
	c.request("DELETE", providerPath, nil, 409, &problem)
	if problem.RelatedAgents == nil || len(*problem.RelatedAgents) != 1 || (*problem.RelatedAgents)[0].Id != a.Id {
		t.Fatal("delete conflict lacks related assistant")
	}
	c.request("DELETE", path, nil, 204, nil)
	c.request("GET", path, nil, 404, nil)
	c.request("POST", "/v1/agents", body, 201, &a)
	c.request("DELETE", "/v1/agents/"+a.Id.String(), nil, 204, nil)
	c.request("DELETE", providerPath, nil, 204, nil)
	var count int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM agents WHERE archived_at IS NOT NULL AND provider_id IS NULL").Scan(&count); err != nil || count != 2 {
		t.Fatal("archived history did not survive provider deletion")
	}
	var agents gen.AgentPage
	c.request("GET", "/v1/agents", nil, 200, &agents)
	if len(agents.Items) != 0 {
		t.Fatal("archived assistant exposed")
	}
}

func TestCatalogWiringFallbackAndEgress(t *testing.T) {
	server, pool := identityTestServer(t)
	c := newCatalogHTTP(t, server)
	upstream := newCatalogUpstream(t, "openai_compatible")
	upstream.unsupported.Store(true)
	p := c.provider("openai_compatible", upstream.server.URL, "Fallback")
	path := "/v1/providers/" + p.Id.String()
	var result gen.ProviderTestResult
	c.request("POST", path+"/test", nil, 200, &result)
	if !result.Ok || upstream.models.Load() != 1 || upstream.streams.Load() != 1 {
		t.Fatal("one-token fallback did not run exactly once")
	}
	upstream.reject.Store(true)
	c.request("POST", path+"/test", nil, 200, &result)
	if result.Ok || result.Code.MustGet() != "provider_auth_failed" || upstream.streams.Load() != 1 {
		t.Fatal("authentication failure triggered fallback")
	}
	// Simulate a persisted endpoint from an older configuration, so runtime guarding is exercised.
	if _, err := pool.Exec(t.Context(), "UPDATE llm_providers SET base_url='http://169.254.169.254/latest/meta-data' WHERE id=$1", p.Id); err != nil {
		t.Fatal("set legacy endpoint")
	}
	c.request("POST", path+"/test", nil, 200, &result)
	if result.Ok || result.Code.MustGet() != "validation_failed" {
		t.Fatal("metadata endpoint not blocked")
	}
	if upstream.models.Load() != 2 {
		t.Fatal("blocked endpoint reached upstream")
	}
}

func TestCatalogWiringConcurrentDeleteAndCreate(t *testing.T) {
	server, pool := identityTestServer(t)
	c := newCatalogHTTP(t, server)
	upstream := newCatalogUpstream(t, "openai_compatible")
	for i := range 5 {
		p := c.provider("openai_compatible", upstream.server.URL, "Concurrent "+string(rune('A'+i)))
		start := make(chan struct{})
		var group sync.WaitGroup
		var createStatus, deleteStatus int
		group.Add(2)
		go func() {
			defer group.Done()
			<-start
			status, _, err := catalogExchange(server, c.token, "POST", "/v1/agents", map[string]any{"name": p.Name, "provider_id": p.Id, "model": "alpha"})
			if err == nil {
				createStatus = status
			}
		}()
		go func() {
			defer group.Done()
			<-start
			status, _, err := catalogExchange(server, c.token, "DELETE", "/v1/providers/"+p.Id.String(), nil)
			if err == nil {
				deleteStatus = status
			}
		}()
		close(start)
		group.Wait()
		validOutcome := (createStatus == 201 && deleteStatus == 409) || (createStatus == 404 && deleteStatus == 204)
		if !validOutcome {
			t.Fatalf("unsafe concurrent outcomes: create=%d delete=%d", createStatus, deleteStatus)
		}
	}
	var count int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM agents a LEFT JOIN llm_providers p ON p.id=a.provider_id WHERE a.archived_at IS NULL AND p.id IS NULL").Scan(&count); err != nil || count != 0 {
		t.Fatal("active assistant orphaned")
	}
}
