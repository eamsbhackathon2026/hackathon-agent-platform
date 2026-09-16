// Package greennode adapts GreenNode MaaS through its OpenAI-compatible API.
package greennode

import (
	"context"
	"net/http"

	"agent-platform/services/agent-service/internal/adapters/outbound/llm/openaicompat"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// BaseURL is the managed GreenNode MaaS endpoint.
const BaseURL = "https://maas-llm-aiplatform-hcm.api.vngcloud.vn/v1"

var supportedModels = []string{
	"z-ai/glm-5.2-hackathon",
	"qwen/qwen3.6-flash",
}

// Client limits model discovery while delegating the wire protocol to the
// shared OpenAI-compatible adapter.
type Client struct{ delegate outbound.LLMClient }

var _ outbound.LLMClient = (*Client)(nil)

// New creates a GreenNode client with the application's guarded transport.
func New(ctx context.Context, conn domain.ProviderConnection, transport *http.Client) (*Client, error) {
	delegate, err := openaicompat.New(ctx, conn, transport)
	if err != nil {
		return nil, err
	}
	return &Client{delegate: delegate}, nil
}

// Stream uses GreenNode's OpenAI-compatible chat completions endpoint.
func (c *Client) Stream(ctx context.Context, req domain.LLMRequest, onDelta func(domain.LLMDelta)) (domain.LLMResult, error) {
	return c.delegate.Stream(ctx, req, onDelta)
}

// ListModels authenticates against GreenNode, then returns the curated models
// enabled by this product instead of exposing the provider's entire catalog.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	if _, err := c.delegate.ListModels(ctx); err != nil {
		return nil, err
	}
	return append([]string(nil), supportedModels...), nil
}
