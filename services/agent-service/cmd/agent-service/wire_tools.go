package main

import (
	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/adapters/outbound/tools/httptool"
	"agent-platform/services/agent-service/internal/adapters/outbound/tools/mcpclient"
	"agent-platform/services/agent-service/internal/config"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/core/services/tooling"
	"agent-platform/services/agent-service/internal/platform/clock"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

func wireTools(cfg config.Config, store *postgres.Store, cipher outbound.SecretCipher, realClock outbound.Clock) (*httpadapter.ToolingHandler, outbound.ToolResolver, error) {
	guard, err := netguard.New(netguard.Config{Development: cfg.AppEnv == "dev", Allowlist: cfg.EgressAllowlist})
	if err != nil {
		return nil, nil, err
	}
	httpInvoker, err := httptool.New(guard.NewHTTPClient())
	if err != nil {
		return nil, nil, err
	}
	mcpConnector, err := mcpclient.New(guard.NewHTTPClient())
	if err != nil {
		return nil, nil, err
	}
	service, err := tooling.NewService(tooling.Dependencies{Tools: store, Connections: store, MCPServers: store, Bindings: store, Agents: store, Tx: store, Cipher: cipher, HTTP: httpInvoker, MCP: mcpConnector, URLs: guard, Clock: realClock, IDs: clock.UUIDv7IDGenerator{}})
	if err != nil {
		return nil, nil, err
	}
	return httpadapter.NewToolingHandler(service, service, service, service), service, nil
}
