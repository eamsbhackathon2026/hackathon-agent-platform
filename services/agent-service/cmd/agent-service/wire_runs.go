package main

import (
	"log/slog"

	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/outbound/llm"
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/config"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/core/services/runs"
	"agent-platform/services/agent-service/internal/platform/clock"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

func wireRuns(cfg config.Config, store *postgres.Store, cipher outbound.SecretCipher, realClock outbound.Clock, tools outbound.ToolResolver, skills outbound.SkillResolver, log *slog.Logger) (*httpadapter.RunHandler, *httpadapter.SessionHandler, *runs.Service, *netguard.Guard, error) {
	guard, err := netguard.New(netguard.Config{Development: cfg.AppEnv == "dev", Allowlist: cfg.EgressAllowlist})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	factory, err := llm.NewFactory(guard)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	service, err := runs.NewService(runs.Dependencies{Agents: store, Providers: store, Sessions: store, Messages: store, Snapshots: store, Runs: store, Spans: store, Tx: store, Cipher: cipher, Factory: factory, Tools: tools, Skills: skills, Queue: store, Idempotency: store, Deliveries: store, WebhookURLs: guard, Clock: realClock, IDs: clock.UUIDv7IDGenerator{}, Logger: log})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return httpadapter.NewRunHandler(service), httpadapter.NewSessionHandler(service), service, guard, nil
}
