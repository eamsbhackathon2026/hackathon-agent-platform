package http_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/services/identity"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

type identityFixture struct {
	handler  http.Handler
	contract routers.Router
	clock    *fakes.Clock
	logs     *bytes.Buffer
}

func newIdentityFixture(t *testing.T, secure bool) identityFixture {
	t.Helper()
	db := fakes.NewStore()
	clock := &fakes.Clock{Time: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}
	service, err := identity.NewService(identity.Dependencies{Users: db, RefreshTokens: db, APIKeys: db, Tx: db, Passwords: fakes.PasswordHasher{}, Tokens: fakes.Tokens{Clock: clock}, KeyHasher: fakes.KeyHasher{}, Cipher: fakes.Cipher{}, Clock: clock, IDs: fakes.IDs{}, Random: fakes.Random{}})
	if err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	handler, err := httpadapter.NewRouter(&httpadapter.Handler{SystemHandler: httpadapter.NewSystemHandler(nil), IdentityHandler: httpadapter.NewIdentityHandler(service, service, service, clock, secure)}, httpadapter.RouterOptions{Resolver: service, Origins: []string{"http://localhost:5173"}, Logger: slog.New(slog.NewJSONHandler(logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	spec.Servers = nil
	contract, err := legacy.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	return identityFixture{handler, contract, clock, logs}
}

func (f identityFixture) request(t *testing.T, method, path, body string, headers map[string]string, want int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, req)
	if response.Code != want {
		t.Fatalf("%s %s: status %d, want %d; body %s", method, path, response.Code, want, response.Body.String())
	}
	if req.Method != http.MethodOptions {
		route, params, err := f.contract.FindRoute(req)
		if err != nil {
			t.Fatal(err)
		}
		input := (&openapi3filter.ResponseValidationInput{RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, Route: route, PathParams: params}, Status: response.Code, Header: response.Header(), Options: &openapi3filter.Options{IncludeResponseStatus: true}}).SetBodyBytes(response.Body.Bytes())
		if err := openapi3filter.ValidateResponse(req.Context(), input); err != nil {
			t.Fatalf("response contract %s %s: %v", method, path, err)
		}
	}
	if want >= 400 {
		problem := decodeIdentity[struct {
			Code string `json:"code"`
		}](t, response)
		codes := map[int]string{400: "validation_failed", 401: "unauthenticated", 403: "forbidden"}
		if code, ok := codes[want]; ok && problem.Code != code {
			t.Fatalf("problem code=%s want=%s", problem.Code, code)
		}
	}
	return response
}

func decodeIdentity[T any](t *testing.T, r *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func identityJSON(t *testing.T, body any) string {
	t.Helper()
	v, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(v)
}

func identityBearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func identityCookie(t *testing.T, r *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range r.Result().Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	t.Fatal("missing refresh cookie")
	return nil
}

func (f identityFixture) owner(t *testing.T) (*httptest.ResponseRecorder, gen.TokenResponse) {
	t.Helper()
	r := f.request(t, "POST", "/v1/auth/register", `{"name":"Owner","email":"owner@example.com","password":"password12345"}`, nil, 201)
	return r, decodeIdentity[gen.TokenResponse](t, r)
}
