// Package llm selects the provider adapter and applies the shared egress policy.
package llm

import (
	"context"
	"errors"
	"net/http"

	"agent-platform/services/agent-service/internal/adapters/outbound/llm/gemini"
	"agent-platform/services/agent-service/internal/adapters/outbound/llm/greennode"
	"agent-platform/services/agent-service/internal/adapters/outbound/llm/openaicompat"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

// Factory shares a guarded transport while keeping SDK credentials request-scoped.
type Factory struct {
	guard  *netguard.Guard
	client *http.Client
}

// NewFactory requires an explicit egress policy for every provider request.
func NewFactory(guard *netguard.Guard) (*Factory, error) {
	if guard == nil {
		return nil, errors.New("egress guard is required")
	}
	return &Factory{guard: guard, client: guard.NewHTTPClient()}, nil
}

// New constructs a client without contacting the upstream service.
func (f *Factory) New(ctx context.Context, conn domain.ProviderConnection) (outbound.LLMClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if conn.APIKey == "" {
		return nil, domain.ErrProviderNotConfigured
	}
	if conn.Kind != domain.ProviderGemini && conn.Kind != domain.ProviderGreenNode && conn.Kind != domain.ProviderOpenAICompatible {
		return nil, domain.ErrProviderBadRequest
	}
	if conn.Kind == domain.ProviderGemini && conn.BaseURL == "" {
		conn.BaseURL = "https://generativelanguage.googleapis.com"
	}
	if conn.Kind == domain.ProviderGreenNode {
		conn.BaseURL = greennode.BaseURL
	}
	if conn.BaseURL == "" {
		return nil, domain.ErrProviderNotConfigured
	}
	if err := f.guard.ValidateURL(conn.BaseURL); err != nil {
		return nil, err
	}
	if conn.Kind == domain.ProviderGemini {
		return gemini.New(ctx, conn, f.client)
	}
	if conn.Kind == domain.ProviderGreenNode {
		return greennode.New(ctx, conn, f.client)
	}
	return openaicompat.New(ctx, conn, f.client)
}

var _ outbound.LLMClientFactory = (*Factory)(nil)
