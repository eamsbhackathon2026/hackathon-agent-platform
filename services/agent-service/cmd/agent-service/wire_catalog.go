package main

import (
	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/outbound/llm"
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/config"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/core/services/catalog"
	"agent-platform/services/agent-service/internal/platform/clock"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

func wireCatalog(cfg config.Config, store *postgres.Store, cipher outbound.SecretCipher, realClock outbound.Clock) (*httpadapter.CatalogHandler, error) {
	guard, err := netguard.New(netguard.Config{Development: cfg.AppEnv == "dev", Allowlist: cfg.EgressAllowlist})
	if err != nil {
		return nil, err
	}
	factory, err := llm.NewFactory(guard)
	if err != nil {
		return nil, err
	}
	service, err := catalog.NewService(catalog.Dependencies{Providers: store, Agents: store, Tx: store, Cipher: cipher, Factory: factory, URLs: guard, Clock: realClock, IDs: clock.UUIDv7IDGenerator{}})
	if err != nil {
		return nil, err
	}
	return httpadapter.NewCatalogHandler(service, service), nil
}
