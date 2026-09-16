package main

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/adapters/outbound/security"
	"agent-platform/services/agent-service/internal/config"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/core/services/identity"
	"agent-platform/services/agent-service/internal/core/services/runs"
	"agent-platform/services/agent-service/internal/platform/clock"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

type application struct {
	handler       http.Handler
	runBackground func(context.Context)
}

func wireApplication(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (http.Handler, error) {
	app, err := wireApplicationRuntime(cfg, pool, log)
	if err != nil {
		return nil, err
	}
	return app.handler, nil
}

func wireApplicationRuntime(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (application, error) {
	jwtKey, err := base64.StdEncoding.DecodeString(cfg.JWTSigningKey)
	if err != nil {
		return application{}, err
	}
	encryptionKey, err := base64.StdEncoding.DecodeString(cfg.EncryptionKey)
	if err != nil {
		return application{}, err
	}
	pepper, err := base64.StdEncoding.DecodeString(cfg.APIKeyPepper)
	if err != nil {
		return application{}, err
	}
	passwords, err := security.NewArgon2idPasswordHasher(security.DefaultArgon2idConfig())
	if err != nil {
		return application{}, err
	}
	realClock := clock.RealClock{}
	tokens, err := security.NewJWTAccessTokenIssuer(jwtKey, cfg.JWTIssuer, cfg.JWTAudience, realClock)
	if err != nil {
		return application{}, err
	}
	keyHasher, err := security.NewHMACAPIKeyHasher(pepper)
	if err != nil {
		return application{}, err
	}
	cipher, err := security.NewAESGCMSecretCipher(encryptionKey)
	if err != nil {
		return application{}, err
	}
	store := postgres.NewStore(pool)
	service, err := identity.NewService(identity.Dependencies{Users: store, RefreshTokens: store, APIKeys: store, Tx: store, Passwords: passwords, Tokens: tokens, KeyHasher: keyHasher, Cipher: cipher, Clock: realClock, IDs: clock.UUIDv7IDGenerator{}, Random: security.NewRandomToken()})
	if err != nil {
		return application{}, err
	}
	handler := &httpadapter.Handler{SystemHandler: httpadapter.NewSystemHandler(pool.Ping), IdentityHandler: httpadapter.NewIdentityHandler(service, service, service, realClock, cfg.AppEnv != "dev")}
	handler.CatalogHandler, err = wireCatalog(cfg, store, cipher, realClock)
	if err != nil {
		return application{}, err
	}
	var toolResolver outbound.ToolResolver
	handler.ToolingHandler, toolResolver, err = wireTools(cfg, store, cipher, realClock)
	if err != nil {
		return application{}, err
	}
	var skillResolver outbound.SkillResolver
	handler.SkillHandler, skillResolver, err = wireSkills(store, realClock)
	if err != nil {
		return application{}, err
	}
	var runService *runs.Service
	var guard *netguard.Guard
	handler.RunHandler, handler.SessionHandler, runService, guard, err = wireRuns(cfg, store, cipher, realClock, toolResolver, skillResolver, log)
	if err != nil {
		return application{}, err
	}
	webhookHandler, workers, err := wireAsync(cfg, store, cipher, realClock, runService, guard, log)
	if err != nil {
		return application{}, err
	}
	handler.WebhookDeliveryHandler = webhookHandler
	router, err := httpadapter.NewRouter(handler, httpadapter.RouterOptions{Resolver: service, Origins: cfg.CORSOrigins, Logger: log})
	if err != nil {
		return application{}, err
	}
	runBackground := func(ctx context.Context) {
		if cfg.WorkerEnabled {
			workers.Run(ctx)
		}
	}
	return application{handler: router, runBackground: runBackground}, nil
}
