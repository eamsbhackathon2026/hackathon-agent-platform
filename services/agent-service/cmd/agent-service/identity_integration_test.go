//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"agent-platform/services/agent-service/internal/config"
	"agent-platform/services/agent-service/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const identityOrigin = "http://localhost:5173"

type identityTestUser struct {
	ID                 string `json:"id"`
	Role               string `json:"role"`
	MustChangePassword bool   `json:"must_change_password"`
}

type identityTestResponse struct {
	SignupAllowed bool             `json:"signup_allowed"`
	AccessToken   string           `json:"access_token"`
	User          identityTestUser `json:"user"`
	Me            struct {
		User identityTestUser `json:"user"`
	} `json:"me"`
	Member            identityTestUser `json:"member"`
	TemporaryPassword string           `json:"temporary_password"`
	Key               string           `json:"key"`
	WebhookSecret     string           `json:"webhook_secret"`
	APIKey            struct {
		ID string `json:"id"`
	} `json:"api_key"`
}

func identityTestServer(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32))
	pool := pgtest.NewPool(t)
	app, err := wireApplicationRuntime(config.Config{AppEnv: "dev", JWTSigningKey: key, EncryptionKey: key, APIKeyPepper: key, JWTIssuer: "identity-integration", JWTAudience: "identity-test", CORSOrigins: []string{identityOrigin}, WorkerEnabled: true, WorkerConcurrency: 2}, pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("wire identity: %v", err)
	}
	workerContext, stopWorkers := context.WithCancel(t.Context())
	workersDone := make(chan struct{})
	go func() { defer close(workersDone); app.runBackground(workerContext) }()
	t.Cleanup(func() { stopWorkers(); <-workersDone })
	server := httptest.NewServer(app.handler)
	t.Cleanup(server.Close)
	return server, pool
}

func identityRequest(t *testing.T, server *httptest.Server, method, path string, body any, token string, cookie *http.Cookie, apiKey string, want int) (identityTestResponse, *http.Cookie) {
	t.Helper()
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal("encode test request")
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, server.URL+path, payload)
	if err != nil {
		t.Fatal("create test request")
	}
	req.Header.Set("Origin", identityOrigin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s transport failed", method, path)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error("close response body")
		}
	}()
	if response.StatusCode != want {
		var problem struct {
			Code  string `json:"code"`
			Title string `json:"title"`
		}
		_ = json.NewDecoder(response.Body).Decode(&problem)
		t.Fatalf("%s %s status=%d want=%d problem=%s title=%s", method, path, response.StatusCode, want, problem.Code, problem.Title)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal("read response body")
	}
	if bytes.Contains(data, []byte(`"tenant`)) {
		t.Fatal("response contains tenant fields")
	}
	if method == http.MethodGet && (path == "/v1/api-keys" || path == "/v1/members") {
		for _, field := range []string{`"key"`, `"key_hash"`, `"password_hash"`, `"temporary_password"`, `"webhook_secret"`, `"webhook_secret_ciphertext"`} {
			if bytes.Contains(data, []byte(field)) {
				t.Fatalf("list exposes forbidden field %s", field)
			}
		}
	}
	var result identityTestResponse
	if len(data) > 0 {
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal("decode response")
		}
	}
	var refresh *http.Cookie
	for _, candidate := range response.Cookies() {
		if candidate.Name == "refresh_token" {
			refresh = candidate
		}
	}
	return result, refresh
}

func TestIdentityWiringLifecycle(t *testing.T) {
	server, _ := identityTestServer(t)
	request := func(method, path string, body any, token string, cookie *http.Cookie, key string, want int) (identityTestResponse, *http.Cookie) {
		return identityRequest(t, server, method, path, body, token, cookie, key, want)
	}
	state, _ := request("GET", "/v1/auth/config", nil, "", nil, "", 200)
	if !state.SignupAllowed {
		t.Fatal("initial signup disabled")
	}
	credentials := map[string]string{"name": "Owner", "email": "owner@example.com", "password": "owner-password-123"}
	owner, cookie := request("POST", "/v1/auth/register", credentials, "", nil, "", 201)
	if owner.Me.User.Role != "owner" || owner.AccessToken == "" {
		t.Fatal("registration did not create owner session")
	}
	if cookie == nil || !cookie.HttpOnly || cookie.Secure || cookie.Path != "/v1/auth" || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge <= 0 {
		t.Fatal("invalid dev refresh cookie flags")
	}
	request("POST", "/v1/auth/register", map[string]string{"name": "Second", "email": "second@example.com", "password": "second-password-123"}, "", nil, "", 403)
	state, _ = request("GET", "/v1/auth/config", nil, "", nil, "", 200)
	if state.SignupAllowed {
		t.Fatal("signup remains enabled")
	}
	loginBody := map[string]string{"email": credentials["email"], "password": credentials["password"]}
	owner, _ = request("POST", "/v1/auth/login", loginBody, "", nil, "", 200)
	me, _ := request("GET", "/v1/me", nil, owner.AccessToken, nil, "", 200)
	if me.User.ID != owner.Me.User.ID {
		t.Fatal("wrong current user")
	}
	_, oldRefresh := request("POST", "/v1/auth/login", loginBody, "", nil, "", 200)
	rotated, nextRefresh := request("POST", "/v1/auth/refresh", nil, "", oldRefresh, "", 200)
	if oldRefresh == nil || nextRefresh == nil || oldRefresh.Value == nextRefresh.Value {
		t.Fatal("refresh did not rotate")
	}
	request("POST", "/v1/auth/refresh", nil, "", oldRefresh, "", 401)
	request("POST", "/v1/auth/refresh", nil, "", nextRefresh, "", 401)
	created, _ := request("POST", "/v1/members", map[string]string{"name": "Member", "email": "member@example.com", "role": "member"}, owner.AccessToken, nil, "", 201)
	if created.TemporaryPassword == "" || !created.Member.MustChangePassword {
		t.Fatal("missing temporary credentials")
	}
	member, _ := request("POST", "/v1/auth/login", map[string]string{"email": "member@example.com", "password": created.TemporaryPassword}, "", nil, "", 200)
	if !member.Me.User.MustChangePassword {
		t.Fatal("temporary login flag missing")
	}
	request("POST", "/v1/me/password", map[string]string{"current_password": created.TemporaryPassword, "new_password": "member-password-456"}, member.AccessToken, nil, "", 204)
	member, _ = request("POST", "/v1/auth/login", map[string]string{"email": "member@example.com", "password": "member-password-456"}, "", nil, "", 200)
	if member.Me.User.MustChangePassword {
		t.Fatal("password change flag not cleared")
	}
	keyBody := map[string]any{"name": "Automation", "scopes": []string{"runs:read"}}
	request("POST", "/v1/api-keys", keyBody, member.AccessToken, nil, "", 403)
	key, _ := request("POST", "/v1/api-keys", keyBody, owner.AccessToken, nil, "", 201)
	if !strings.HasPrefix(key.Key, "apk_") || !strings.HasPrefix(key.WebhookSecret, "whsec_") || key.APIKey.ID == "" {
		t.Fatal("key secrets missing")
	}
	request("GET", "/v1/api-keys", nil, owner.AccessToken, nil, "", 200)
	request("GET", "/v1/members", nil, owner.AccessToken, nil, "", 200)
	// A valid key resolves successfully, then is forbidden on the user-only route.
	request("GET", "/v1/me", nil, "", nil, key.Key, 403)
	request("DELETE", "/v1/api-keys/"+key.APIKey.ID, nil, owner.AccessToken, nil, "", 204)
	request("GET", "/v1/me", nil, "", nil, key.Key, 401)
	// Refresh revocation does not blacklist access JWTs before their 15-minute expiry.
	request("GET", "/v1/me", nil, rotated.AccessToken, nil, "", 200)
	request("PATCH", "/v1/members/"+member.Me.User.ID, map[string]string{"status": "disabled"}, owner.AccessToken, nil, "", 200)
	request("GET", "/v1/me", nil, member.AccessToken, nil, "", 401)
}

func TestIdentityConcurrentFirstRegistration(t *testing.T) {
	server, pool := identityTestServer(t)
	start := make(chan struct{})
	statuses := make(chan int, 2)
	var group sync.WaitGroup
	for _, email := range []string{"first@example.com", "second@example.com"} {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			body, err := json.Marshal(map[string]string{"name": "Owner", "email": email, "password": "parallel-password-123"})
			if err != nil {
				statuses <- 0
				return
			}
			response, err := server.Client().Post(server.URL+"/v1/auth/register", "application/json", bytes.NewReader(body))
			if err != nil {
				statuses <- 0
				return
			}
			if err := response.Body.Close(); err != nil {
				statuses <- 0
				return
			}
			statuses <- response.StatusCode
		}()
	}
	close(start)
	group.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[201] != 1 || counts[403] != 1 {
		t.Fatalf("parallel registration status counts: %v", counts)
	}
	var owners int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM users WHERE role='owner' AND status='active'").Scan(&owners); err != nil {
		t.Fatal("count owners")
	}
	if owners != 1 {
		t.Fatalf("active owners=%d want=1", owners)
	}
}

func TestIdentityConcurrentLastOwnerDowngrade(t *testing.T) {
	server, pool := identityTestServer(t)
	request := func(method, path string, body any, token string) (identityTestResponse, *http.Cookie) {
		return identityRequest(t, server, method, path, body, token, nil, "", map[string]int{"/v1/auth/register": 201, "/v1/members": 201, "/v1/auth/login": 200, "/v1/me/password": 204}[path])
	}
	first, _ := request("POST", "/v1/auth/register", map[string]string{"name": "First", "email": "first@example.com", "password": "first-password-123"}, "")
	created, _ := request("POST", "/v1/members", map[string]string{"name": "Second", "email": "second@example.com", "role": "owner"}, first.AccessToken)
	second, _ := request("POST", "/v1/auth/login", map[string]string{"email": "second@example.com", "password": created.TemporaryPassword}, "")
	request("POST", "/v1/me/password", map[string]string{"current_password": created.TemporaryPassword, "new_password": "second-password-456"}, second.AccessToken)
	second, _ = request("POST", "/v1/auth/login", map[string]string{"email": "second@example.com", "password": "second-password-456"}, "")
	start := make(chan struct{})
	statuses := make(chan int, 2)
	var group sync.WaitGroup
	for _, owner := range []identityTestResponse{first, second} {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			req, err := http.NewRequest("PATCH", server.URL+"/v1/members/"+owner.Me.User.ID, strings.NewReader(`{"role":"member"}`))
			if err != nil {
				statuses <- 0
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+owner.AccessToken)
			response, err := server.Client().Do(req)
			if err != nil {
				statuses <- 0
				return
			}
			if err = response.Body.Close(); err != nil {
				statuses <- 0
				return
			}
			statuses <- response.StatusCode
		}()
	}
	close(start)
	group.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[200] != 1 || counts[403]+counts[409] != 1 {
		t.Fatalf("parallel downgrade status counts: %v", counts)
	}
	var owners int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM users WHERE role='owner' AND status='active'").Scan(&owners); err != nil {
		t.Fatal("count owners")
	}
	if owners != 1 {
		t.Fatalf("active owners=%d want=1", owners)
	}
}
