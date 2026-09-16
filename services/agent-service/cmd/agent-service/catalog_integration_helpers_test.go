//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

const catalogGoodKey = "catalog-test-credential-good"
const catalogBadKey = "catalog-test-credential-bad"
const catalogRawError = "upstream-private-diagnostic"

type catalogHTTP struct {
	t      *testing.T
	server *httptest.Server
	token  string
}

func newCatalogHTTP(t *testing.T, server *httptest.Server) catalogHTTP {
	t.Helper()
	owner, _ := identityRequest(t, server, "POST", "/v1/auth/register", map[string]string{"name": "Owner", "email": "catalog@example.com", "password": "catalog-password-123"}, "", nil, "", 201)
	return catalogHTTP{t, server, owner.AccessToken}
}
func (c catalogHTTP) request(method, path string, body any, want int, out any) {
	c.t.Helper()
	status, data, err := catalogExchange(c.server, c.token, method, path, body)
	if err != nil {
		c.t.Fatal("catalog request failed")
	}
	if status != want {
		c.t.Fatalf("%s %s status=%d want=%d", method, path, status, want)
	}
	for _, secret := range []string{catalogGoodKey, catalogBadKey, catalogRawError, "tool-private-value", "mcp-private-value", `"api_key_ciphertext"`, `"api_key":`} {
		if bytes.Contains(data, []byte(secret)) {
			c.t.Fatal("catalog response exposed private data")
		}
	}
	if want >= 400 {
		var problem gen.Problem
		if json.Unmarshal(data, &problem) != nil || problem.Status != want || problem.Code == "" || problem.Title == "" {
			c.t.Fatal("invalid problem response")
		}
	}
	if out != nil && json.Unmarshal(data, out) != nil {
		c.t.Fatal("decode catalog response")
	}
}
func catalogExchange(server *httptest.Server, token, method, path string, body any) (int, []byte, error) {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, server.URL+path, payload)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", identityOrigin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}
func (c catalogHTTP) provider(kind, url, name string) gen.Provider {
	c.t.Helper()
	var p gen.Provider
	c.request("POST", "/v1/providers", map[string]any{"name": name, "kind": kind, "base_url": url, "api_key": catalogGoodKey, "default_model": "alpha"}, 201, &p)
	if p.Id.String() == "00000000-0000-0000-0000-000000000000" || p.Status != "unchecked" || p.CreatedAt.IsZero() || p.ApiKeyHint.MustGet() != "good" {
		c.t.Fatal("invalid provider fields")
	}
	return p
}

type catalogUpstream struct {
	server              *httptest.Server
	models, streams     atomic.Int32
	reject, unsupported atomic.Bool
}

func newCatalogUpstream(t *testing.T, kind string) *catalogUpstream {
	t.Helper()
	u := &catalogUpstream{}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Goog-Api-Key")
		if kind == "openai_compatible" {
			key = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		if key != catalogGoodKey && key != catalogBadKey {
			t.Error("upstream received unexpected credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/models") {
			u.models.Add(1)
			if u.reject.Load() || key == catalogBadKey {
				w.WriteHeader(401)
				_, _ = io.WriteString(w, `{"error":{"code":401,"status":"UNAUTHENTICATED","message":"`+catalogRawError+" "+catalogBadKey+`"}}`)
				return
			}
			if u.unsupported.Load() {
				w.WriteHeader(404)
				_, _ = io.WriteString(w, `{"error":{"message":"unsupported"}}`)
				return
			}
			if kind == "gemini" {
				_, _ = io.WriteString(w, `{"models":[{"name":"models/zeta","supportedGenerationMethods":["generateContent"]},{"name":"models/alpha"},{"name":"models/alpha"}]}`)
			} else {
				_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"zeta","object":"model"},{"id":"alpha","object":"model"},{"id":"alpha","object":"model"}]}`)
			}
			return
		}
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			u.streams.Add(1)
			var request struct {
				MaxTokens int  `json:"max_completion_tokens"`
				Stream    bool `json:"stream"`
			}
			if json.NewDecoder(r.Body).Decode(&request) != nil || request.MaxTokens != 1 || !request.Stream {
				t.Error("fallback did not request one streamed token")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"id\":\"completion\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"alpha\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		t.Error("unexpected upstream route")
		w.WriteHeader(404)
	}))
	t.Cleanup(u.server.Close)
	return u
}

func (c catalogHTTP) member() catalogHTTP {
	c.t.Helper()
	created, _ := identityRequest(c.t, c.server, "POST", "/v1/members", map[string]string{"name": "Member", "email": "catalog-member@example.com", "role": "member"}, c.token, nil, "", 201)
	session, _ := identityRequest(c.t, c.server, "POST", "/v1/auth/login", map[string]string{"email": "catalog-member@example.com", "password": created.TemporaryPassword}, "", nil, "", 200)
	identityRequest(c.t, c.server, "POST", "/v1/me/password", map[string]string{"current_password": created.TemporaryPassword, "new_password": "member-password-123"}, session.AccessToken, nil, "", 204)
	c.token = session.AccessToken
	return c
}
