// Package gemini implements the provider-neutral LLM port with the Gemini SDK.
package gemini

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"google.golang.org/genai"
)

// Client performs Gemini generation and model discovery through a guarded transport.
type Client struct{ sdk *genai.Client }

var _ outbound.LLMClient = (*Client)(nil)

// New requires a guarded HTTP client; credentials and endpoint never fall back to environment.
func New(ctx context.Context, conn domain.ProviderConnection, httpClient *http.Client) (*Client, error) {
	if httpClient == nil || strings.TrimSpace(conn.APIKey) == "" {
		return nil, domain.ErrProviderNotConfigured
	}
	baseURL := conn.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/"
	}
	attempts := int32(1)
	safeClient := *httpClient
	transport := safeClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	safeClient.Transport = safeBodyTransport{base: transport}
	sdk, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: conn.APIKey, Backend: genai.BackendGeminiAPI, HTTPClient: &safeClient,
		HTTPOptions: genai.HTTPOptions{BaseURL: baseURL, RetryOptions: &genai.HTTPRetryOptions{Attempts: &attempts}},
	})
	if err != nil {
		return nil, safeError(ctx, err, false)
	}
	return &Client{sdk: sdk}, nil
}

// ListModels collects every page, excluding models with no generation support.
func (c *Client) ListModels(ctx context.Context) (models []string, err error) {
	// Models.All eagerly loads page one; the SDK converter can panic before
	// returning its iterator as well as while loading subsequent pages.
	defer func() {
		if recover() != nil {
			models, err = nil, domain.ErrProviderBadRequest
		}
	}()
	models = []string{}
	for model, err := range c.sdk.Models.All(ctx) {
		if err != nil {
			return nil, safeError(ctx, err, true)
		}
		if model == nil || (len(model.SupportedActions) > 0 && !slices.Contains(model.SupportedActions, "generateContent")) {
			continue
		}
		if name := strings.TrimPrefix(model.Name, "models/"); name != "" {
			models = append(models, name)
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return models, nil
}

func safeError(ctx context.Context, err error, listing bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errContext(err)
	}
	if errors.Is(err, domain.ErrEgressDenied) {
		return domain.ErrEgressDenied
	}
	if errors.Is(err, domain.ErrProviderUnreachable) || errors.Is(err, io.ErrUnexpectedEOF) {
		return domain.ErrProviderUnreachable
	}
	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code == http.StatusBadRequest && invalidAPIKey(apiErr) {
			return domain.ErrProviderAuth
		}
		switch apiErr.Code {
		case 401, 403:
			return domain.ErrProviderAuth
		case 404:
			if listing {
				return domain.ErrModelsUnsupported
			}
			return domain.ErrModelNotFound
		case 405, 501:
			if listing {
				return domain.ErrModelsUnsupported
			}
		case 429:
			return domain.ErrProviderRateLimited
		}
		if apiErr.Code >= 500 {
			return domain.ErrProviderUnreachable
		}
		return domain.ErrProviderBadRequest
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return domain.ErrProviderUnreachable
	}
	return domain.ErrProviderBadRequest
}

func invalidAPIKey(err genai.APIError) bool {
	for _, detail := range err.Details {
		if detail["@type"] == "type.googleapis.com/google.rpc.ErrorInfo" && detail["domain"] == "googleapis.com" && detail["reason"] == "API_KEY_INVALID" {
			return true
		}
	}
	return false
}

func errContext(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return context.DeadlineExceeded
}
