package http_test

import (
	"net/url"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestIdentityMemberPagination(t *testing.T) {
	f := newIdentityFixture(t, false)
	_, owner := f.owner(t)
	headers := identityBearer(owner.AccessToken)
	f.request(t, "POST", "/v1/members", `{"name":"Second","email":"second@example.com","role":"member"}`, headers, 201)
	type page struct {
		Items      []gen.Member `json:"items"`
		NextCursor *string      `json:"next_cursor"`
	}
	first := decodeIdentity[page](t, f.request(t, "GET", "/v1/members?limit=1", "", headers, 200))
	if len(first.Items) != 1 || first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatal("missing next page cursor")
	}
	second := decodeIdentity[page](t, f.request(t, "GET", "/v1/members?limit=1&cursor="+url.QueryEscape(*first.NextCursor), "", headers, 200))
	if len(second.Items) != 1 || second.NextCursor != nil || second.Items[0].Id == first.Items[0].Id {
		t.Fatal("pagination skipped or repeated a member")
	}
}

func TestIdentityRejectsInvalidRequests(t *testing.T) {
	f := newIdentityFixture(t, false)
	_, owner := f.owner(t)
	headers := identityBearer(owner.AccessToken)
	for _, tc := range []struct{ name, method, path, body string }{
		{"malformed JSON", "POST", "/v1/auth/login", `{"email":`},
		{"missing required", "POST", "/v1/auth/login", `{"email":"owner@example.com"}`},
		{"unknown property", "POST", "/v1/auth/login", `{"email":"owner@example.com","password":"password12345","role":"owner"}`},
		{"null password", "POST", "/v1/auth/login", `{"email":"owner@example.com","password":null}`},
		{"unknown member property", "POST", "/v1/members", `{"email":"new@example.com","name":"New","role":"member","tenant_id":"old"}`},
		{"invalid UUID", "PATCH", "/v1/members/not-a-uuid", `{"name":"New"}`},
		{"empty patch", "PATCH", "/v1/members/" + owner.Me.User.Id.String(), `{}`},
		{"null patch", "PATCH", "/v1/members/" + owner.Me.User.Id.String(), `{"name":null}`},
		{"limit zero", "GET", "/v1/members?limit=0", ""},
		{"limit too large", "GET", "/v1/members?limit=101", ""},
		{"malformed cursor", "GET", "/v1/members?cursor=invalid", ""},
		{"unknown scope", "POST", "/v1/api-keys", `{"name":"Key","scopes":["admin"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) { f.request(t, tc.method, tc.path, tc.body, headers, 400) })
	}
	f.request(t, "GET", "/v1/me", "", nil, 401)
	f.request(t, "POST", "/v1/auth/refresh", "", map[string]string{"Cookie": "refresh_token=one; refresh_token=two"}, 400)
}

func TestIdentityCORSAndOriginProtection(t *testing.T) {
	f := newIdentityFixture(t, false)
	r, _ := f.owner(t)
	cookie := "refresh_token=" + identityCookie(t, r).Value
	for _, headers := range []map[string]string{
		{"Origin": "https://evil.example", "Cookie": cookie},
		{"Origin": "null", "Cookie": cookie},
		{"Sec-Fetch-Site": "cross-site", "Cookie": cookie},
	} {
		f.request(t, "POST", "/v1/auth/refresh", "", headers, 403)
	}
	allowed := f.request(t, "POST", "/v1/auth/refresh", "", map[string]string{"Origin": "http://localhost:5173", "Cookie": cookie}, 200)
	if allowed.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" || allowed.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("missing credentialed CORS headers")
	}
	preflight := f.request(t, "OPTIONS", "/v1/agents/00000000-0000-4000-8000-000000000001/skills", "", map[string]string{"Origin": "http://localhost:5173", "Access-Control-Request-Method": "PUT", "Access-Control-Request-Headers": "authorization,content-type"}, 204)
	if !strings.Contains(preflight.Header().Get("Access-Control-Allow-Methods"), "PUT") {
		t.Fatal("missing preflight methods")
	}
	f.request(t, "OPTIONS", "/v1/members", "", map[string]string{"Origin": "https://evil.example", "Access-Control-Request-Method": "POST"}, 403)
	f.request(t, "OPTIONS", "/v1/members", "", map[string]string{"Origin": "http://localhost:5173", "Access-Control-Request-Method": "TRACE"}, 403)
	f.request(t, "OPTIONS", "/v1/members", "", map[string]string{"Origin": "http://localhost:5173", "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "x-unknown"}, 403)
	if decodeIdentity[gen.TokenResponse](t, allowed).AccessToken == "" {
		t.Fatal("allowed refresh missing access token")
	}
}
