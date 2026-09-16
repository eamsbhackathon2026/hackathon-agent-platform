package http

import (
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"context"
)

// ListProviders exposes provider metadata without encrypted or plaintext credentials.
func (h *CatalogHandler) ListProviders(ctx context.Context, r gen.ListProvidersRequestObject) (gen.ListProvidersResponseObject, error) {
	page, err := h.providers.ListProviders(ctx, requestFrom(ctx).principal, pageRequest(r.Params.Limit, r.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.Provider, len(page.Items))
	for i, p := range page.Items {
		items[i] = providerDTO(p)
	}
	return gen.ListProviders200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// GetProvider returns the safe view of one saved connection.
func (h *CatalogHandler) GetProvider(ctx context.Context, r gen.GetProviderRequestObject) (gen.GetProviderResponseObject, error) {
	p, err := h.providers.GetProvider(ctx, requestFrom(ctx).principal, r.ProviderId)
	if err != nil {
		return nil, err
	}
	return gen.GetProvider200JSONResponse(providerDTO(p)), nil
}

// CreateProvider stores a new encrypted connection.
func (h *CatalogHandler) CreateProvider(ctx context.Context, r gen.CreateProviderRequestObject) (gen.CreateProviderResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	p, err := h.providers.CreateProvider(ctx, requestFrom(ctx).principal, inbound.ProviderCreateCommand{Name: r.Body.Name, Kind: domain.ProviderKind(r.Body.Kind), BaseURL: valuePointer(r.Body.BaseUrl), DefaultModel: valuePointer(r.Body.DefaultModel), APIKey: stringValue(r.Body.ApiKey)})
	if err != nil {
		return nil, err
	}
	return gen.CreateProvider201JSONResponse(providerDTO(p)), nil
}

// UpdateProvider preserves omission, clearing, and replacement of secrets.
func (h *CatalogHandler) UpdateProvider(ctx context.Context, r gen.UpdateProviderRequestObject) (gen.UpdateProviderResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	c := inbound.ProviderUpdateCommand{Name: r.Body.Name, BaseURL: changeValue(r.Body.BaseUrl), DefaultModel: changeValue(r.Body.DefaultModel), APIKey: changeValue(r.Body.ApiKey)}
	if r.Body.Kind != nil {
		v := domain.ProviderKind(*r.Body.Kind)
		c.Kind = &v
	}
	p, err := h.providers.UpdateProvider(ctx, requestFrom(ctx).principal, r.ProviderId, c)
	if err != nil {
		return nil, err
	}
	return gen.UpdateProvider200JSONResponse(providerDTO(p)), nil
}

// DeleteProvider rejects connections still used by active assistants.
func (h *CatalogHandler) DeleteProvider(ctx context.Context, r gen.DeleteProviderRequestObject) (gen.DeleteProviderResponseObject, error) {
	err := h.providers.DeleteProvider(ctx, requestFrom(ctx).principal, r.ProviderId)
	return gen.DeleteProvider204Response{}, err
}

// TestProvider reports connection failures as a safe result for the settings screen.
func (h *CatalogHandler) TestProvider(ctx context.Context, r gen.TestProviderRequestObject) (gen.TestProviderResponseObject, error) {
	result, err := h.providers.TestProvider(ctx, requestFrom(ctx).principal, r.ProviderId)
	if err != nil {
		return nil, err
	}
	var code *gen.ProviderTestResultCode
	if result.Failure != nil {
		v := gen.ProviderTestResultCode(result.Failure.Code)
		code = &v
	}
	return gen.TestProvider200JSONResponse{Ok: result.OK, Message: result.Message, Code: nullableValue(code)}, nil
}

// ListProviderModels provides a stable model picker while allowing manual configuration.
func (h *CatalogHandler) ListProviderModels(ctx context.Context, r gen.ListProviderModelsRequestObject) (gen.ListProviderModelsResponseObject, error) {
	page, err := h.providers.ListProviderModels(ctx, requestFrom(ctx).principal, r.ProviderId, pageRequest(r.Params.Limit, r.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.Model, len(page.Items))
	for i, id := range page.Items {
		items[i] = gen.Model{Id: id, DisplayName: id}
	}
	return gen.ListProviderModels200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}
