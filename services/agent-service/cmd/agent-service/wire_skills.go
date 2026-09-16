package main

import (
	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/core/services/skills"
	"agent-platform/services/agent-service/internal/platform/clock"
)

func wireSkills(store *postgres.Store, realClock outbound.Clock) (*httpadapter.SkillHandler, outbound.SkillResolver, error) {
	service, err := skills.NewService(skills.Dependencies{Skills: store, Agents: store, Tx: store, Clock: realClock, IDs: clock.UUIDv7IDGenerator{}})
	if err != nil {
		return nil, nil, err
	}
	return httpadapter.NewSkillHandler(service, service), service, nil
}
