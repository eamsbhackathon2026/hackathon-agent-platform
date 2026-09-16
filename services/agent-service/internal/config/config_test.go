package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"APP_ENV", "production-typo"},
		{"HTTP_ADDR", "localhost"},
		{"HTTP_ADDR", "localhost:70000"},
		{"LOG_LEVEL", "verbose"},
		{"DATABASE_URL", ""},
		{"JWT_SIGNING_KEY", "private-value"},
		{"APP_ENCRYPTION_KEY", ""},
		{"API_KEY_PEPPER", base64.StdEncoding.EncodeToString([]byte("too-short"))},
		{"CORS_ORIGINS", "*"},
		{"CORS_ORIGINS", "https://example.com/path"},
		{"CORS_ORIGINS", "https://example.com?"},
		{"CORS_ORIGINS", "https://user:pass@example.com"},
		{"JWT_ISSUER", " "},
		{"EGRESS_ALLOWLIST", "https://internal.example.com"},
		{"EGRESS_ALLOWLIST", "10.0.0.0/999"},
		{"WORKER_CONCURRENCY", "0"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			validEnvironment(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatal("invalid configuration accepted")
			} else if strings.Contains(err.Error(), "private-value") {
				t.Fatal("secret exposed")
			}
		})
	}
}

func TestProductionRequiresDatabase(t *testing.T) {
	validEnvironment(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("HTTP_ADDR", "127.0.0.1:8080")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("production without a database accepted")
	}
}

func validEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("EGRESS_ALLOWLIST", "")
	for key, value := range map[string]string{
		"APP_ENV": "dev", "HTTP_ADDR": "127.0.0.1:8080", "LOG_LEVEL": "info", "DATABASE_URL": "postgres://localhost/test",
		"JWT_SIGNING_KEY":    base64.StdEncoding.EncodeToString([]byte(strings.Repeat("s", 32))),
		"APP_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("e", 32))),
		"API_KEY_PEPPER":     base64.StdEncoding.EncodeToString([]byte(strings.Repeat("p", 32))),
		"JWT_ISSUER":         "agent-platform", "JWT_AUDIENCE": "agent-platform-admin", "CORS_ORIGINS": "http://localhost:5173",
	} {
		t.Setenv(key, value)
	}
}

func TestLoadValidConfigAndProductionOrigins(t *testing.T) {
	validEnvironment(t)
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "prod")
	if _, err := Load(); err == nil {
		t.Fatal("production HTTP origin accepted")
	}
	t.Setenv("CORS_ORIGINS", "https://example.com, https://admin.example.com")
	cfg, err := Load()
	if err != nil || cfg.CORSOrigins[1] != "https://admin.example.com" {
		t.Fatalf("config=%v", err)
	}
}
