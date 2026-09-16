package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestSystemRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		pingError  error
		status     int
		pingCalled bool
		schema     string
	}{
		{"health ignores database failure", http.MethodGet, "/healthz", errors.New("database unavailable"), 200, false, "HealthStatus"},
		{"ready checks database", http.MethodGet, "/readyz", nil, 200, true, "HealthStatus"},
		{"ready hides database error", http.MethodGet, "/readyz", errors.New("password=private-value"), 503, true, "Problem"},
		{"unknown route", http.MethodGet, "/missing", nil, 404, false, "Problem"},
		{"unsupported method", http.MethodPost, "/healthz", nil, 405, false, "Problem"},
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := httpadapter.NewSystemHandler(func(ctx context.Context) error {
				called = true
				if _, ok := ctx.Deadline(); !ok {
					t.Error("database ping has no deadline")
				}
				return tc.pingError
			})
			response := httptest.NewRecorder()
			newSystemRouter(t, handler).ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			if response.Code != tc.status || called != tc.pingCalled {
				t.Fatalf("status=%d ping=%v; want %d, %v", response.Code, called, tc.status, tc.pingCalled)
			}
			contentType := "application/json"
			if tc.status >= 400 {
				contentType = "application/problem+json"
			}
			if response.Header().Get("Content-Type") != contentType {
				t.Errorf("unexpected content type: %s", response.Header().Get("Content-Type"))
			}
			if strings.Contains(response.Body.String(), "private-value") {
				t.Fatal("database error exposed a secret")
			}
			assertSchema(t, spec.Components.Schemas[tc.schema].Value, response.Body.Bytes())
		})
	}
}

func TestReadinessWithoutConfiguredDatabase(t *testing.T) {
	response := httptest.NewRecorder()
	newSystemRouter(t, httpadapter.NewSystemHandler(nil)).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("standalone process is not ready: %d", response.Code)
	}
}

func TestReadinessPropagatesRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	handler := httpadapter.NewSystemHandler(func(ctx context.Context) error { return ctx.Err() })
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil).WithContext(ctx)
	newSystemRouter(t, handler).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("cancelled ping returned %d", response.Code)
	}
}

func assertSchema(t *testing.T, schema *openapi3.Schema, body []byte) {
	t.Helper()
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if err := schema.VisitJSON(payload); err != nil {
		t.Fatalf("response violates OpenAPI: %v", err)
	}
}

func newSystemRouter(t *testing.T, system *httpadapter.SystemHandler) http.Handler {
	t.Helper()
	router, err := httpadapter.NewRouter(&httpadapter.Handler{SystemHandler: system}, httpadapter.RouterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return router
}
