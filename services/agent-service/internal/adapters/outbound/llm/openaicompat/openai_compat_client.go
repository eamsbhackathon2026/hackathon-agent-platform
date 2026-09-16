// Package openaicompat implements the provider-neutral LLM port using OpenAI-compatible APIs.
package openaicompat

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Client translates the OpenAI-compatible protocol through the supplied guarded transport.
type Client struct{ sdk openai.Client }

var _ outbound.LLMClient = (*Client)(nil)

// New requires an explicit HTTP client with the application's egress policy.
func New(ctx context.Context, conn domain.ProviderConnection, transport *http.Client) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if transport == nil || strings.TrimSpace(conn.BaseURL) == "" {
		return nil, domain.ErrProviderNotConfigured
	}
	return &Client{sdk: openai.NewClient(option.WithBaseURL(conn.BaseURL), option.WithAPIKey(conn.APIKey), option.WithHTTPClient(transport), option.WithMaxRetries(0))}, nil
}

// ListModels returns model IDs; the catalog service sorts and deduplicates them.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	page, err := c.sdk.Models.List(ctx)
	if err != nil {
		return nil, providerError(ctx, err, true)
	}
	ids := make([]string, 0, len(page.Data))
	for _, model := range page.Data {
		if model.ID != "" {
			ids = append(ids, model.ID)
		}
	}
	return ids, nil
}

func providerError(ctx context.Context, err error, listing bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, domain.ErrEgressDenied) {
		return domain.ErrEgressDenied
	}
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		if listing && (apiErr.StatusCode == 404 || apiErr.StatusCode == 405 || apiErr.StatusCode == 501) {
			return domain.ErrModelsUnsupported
		}
		switch apiErr.StatusCode {
		case 401, 403:
			return domain.ErrProviderAuth
		case 429:
			return domain.ErrProviderRateLimited
		case 404:
			return domain.ErrModelNotFound
		case 400, 422:
			return domain.ErrProviderBadRequest
		}
	}
	return domain.ErrProviderUnreachable
}
