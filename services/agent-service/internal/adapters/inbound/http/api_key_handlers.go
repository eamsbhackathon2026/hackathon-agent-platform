package http

import (
	"context"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// ListApiKeys lists key metadata without plaintext secrets.
//
//nolint:revive // Method name must match the generated OpenAPI interface.
func (h *IdentityHandler) ListApiKeys(ctx context.Context, r gen.ListApiKeysRequestObject) (gen.ListApiKeysResponseObject, error) {
	page, err := h.keys.ListAPIKeys(ctx, requestFrom(ctx).principal, pageRequest(r.Params.Limit, r.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.ApiKey, len(page.Items))
	for i, k := range page.Items {
		items[i] = apiKeyDTO(k)
	}
	return gen.ListApiKeys200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// CreateApiKey returns a new key and signing secret once.
//
//nolint:revive // Method name must match the generated OpenAPI interface.
func (h *IdentityHandler) CreateApiKey(ctx context.Context, r gen.CreateApiKeyRequestObject) (gen.CreateApiKeyResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	scopes := make([]string, len(r.Body.Scopes))
	for i, s := range r.Body.Scopes {
		scopes[i] = string(s)
	}
	key, err := h.keys.CreateAPIKey(ctx, requestFrom(ctx).principal, inbound.APIKeyCreateCommand{Name: r.Body.Name, Scopes: scopes})
	if err != nil {
		return nil, err
	}
	return gen.CreateApiKey201JSONResponse{ApiKey: apiKeyDTO(key.APIKey), Key: key.Key, WebhookSecret: key.WebhookSecret}, nil
}

// RevokeApiKey immediately disables a key.
//
//nolint:revive // Method name must match the generated OpenAPI interface.
func (h *IdentityHandler) RevokeApiKey(ctx context.Context, r gen.RevokeApiKeyRequestObject) (gen.RevokeApiKeyResponseObject, error) {
	err := h.keys.RevokeAPIKey(ctx, requestFrom(ctx).principal, r.KeyId)
	return gen.RevokeApiKey204Response{}, err
}
