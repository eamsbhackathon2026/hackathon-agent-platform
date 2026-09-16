package http

import "agent-platform/services/agent-service/internal/core/ports/inbound"

// CatalogHandler translates assistant and provider APIs to catalog use cases.
type CatalogHandler struct {
	providers inbound.ProviderUseCase
	agents    inbound.AgentUseCase
}

// NewCatalogHandler binds the two catalog use cases.
func NewCatalogHandler(providers inbound.ProviderUseCase, agents inbound.AgentUseCase) *CatalogHandler {
	return &CatalogHandler{providers: providers, agents: agents}
}
