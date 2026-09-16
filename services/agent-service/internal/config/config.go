// Package config loads application settings from environment variables.
package config

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"

	"agent-platform/services/agent-service/internal/platform/netguard"
	"github.com/caarlos0/env/v11"
)

// Config contains the process configuration. Secrets must never be logged.
type Config struct {
	HTTPAddr          string   `env:"HTTP_ADDR" envDefault:"127.0.0.1:8080"`
	AppEnv            string   `env:"APP_ENV" envDefault:"dev"`
	DatabaseURL       string   `env:"DATABASE_URL"`
	LogLevel          string   `env:"LOG_LEVEL" envDefault:"info"`
	JWTSigningKey     string   `env:"JWT_SIGNING_KEY"`
	EncryptionKey     string   `env:"APP_ENCRYPTION_KEY"`
	APIKeyPepper      string   `env:"API_KEY_PEPPER"`
	JWTIssuer         string   `env:"JWT_ISSUER" envDefault:"agent-platform"`
	JWTAudience       string   `env:"JWT_AUDIENCE" envDefault:"agent-platform-admin"`
	CORSOrigins       []string `env:"CORS_ORIGINS" envSeparator:","`
	EgressAllowlist   []string `env:"EGRESS_ALLOWLIST" envSeparator:","`
	WorkerEnabled     bool     `env:"WORKER_ENABLED" envDefault:"true"`
	WorkerConcurrency int      `env:"WORKER_CONCURRENCY" envDefault:"4"`
}

// Load reads and validates the environment without exposing connection secrets.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, errors.New("không đọc được cấu hình môi trường")
	}
	if cfg.AppEnv != "dev" && cfg.AppEnv != "test" && cfg.AppEnv != "prod" {
		return Config{}, errors.New("APP_ENV phải là dev, test hoặc prod")
	}
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil {
		return Config{}, errors.New("HTTP_ADDR phải có dạng host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, errors.New("HTTP_ADDR phải có cổng từ 1 đến 65535")
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return Config{}, errors.New("LOG_LEVEL phải là debug, info, warn hoặc error")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL là bắt buộc")
	}
	for _, secret := range []struct{ name, value string }{{"JWT_SIGNING_KEY", cfg.JWTSigningKey}, {"APP_ENCRYPTION_KEY", cfg.EncryptionKey}, {"API_KEY_PEPPER", cfg.APIKeyPepper}} {
		value, err := base64.StdEncoding.DecodeString(secret.value)
		if err != nil || len(value) != 32 {
			return Config{}, errors.New(secret.name + " phải là 32 byte ngẫu nhiên mã hóa Base64")
		}
	}
	if strings.TrimSpace(cfg.JWTIssuer) == "" || strings.TrimSpace(cfg.JWTAudience) == "" {
		return Config{}, errors.New("JWT_ISSUER và JWT_AUDIENCE không được để trống")
	}
	for i, origin := range cfg.CORSOrigins {
		origin = strings.TrimSpace(origin)
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || (u.Scheme != "https" && (cfg.AppEnv == "prod" || u.Scheme != "http")) {
			return Config{}, errors.New("CORS_ORIGINS phải là các origin HTTP(S) chính xác, dùng HTTPS ở prod")
		}
		cfg.CORSOrigins[i] = origin
	}
	if _, err := netguard.New(netguard.Config{Development: cfg.AppEnv == "dev", Allowlist: cfg.EgressAllowlist}); err != nil {
		return Config{}, errors.New("EGRESS_ALLOWLIST phải chứa hostname chính xác hoặc CIDR hợp lệ")
	}
	if cfg.WorkerConcurrency < 1 || cfg.WorkerConcurrency > 64 {
		return Config{}, errors.New("WORKER_CONCURRENCY phải từ 1 đến 64")
	}
	return cfg, nil
}
