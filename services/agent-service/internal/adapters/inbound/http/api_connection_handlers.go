package http

import (
	"context"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// ListAPIConnections handles cursor-paginated reusable API connection retrieval.
func (h *ToolingHandler) ListAPIConnections(ctx context.Context, request gen.ListAPIConnectionsRequestObject) (gen.ListAPIConnectionsResponseObject, error) {
	page, err := h.connections.ListAPIConnections(ctx, requestFrom(ctx).principal, pageRequest(request.Params.Limit, request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.ApiConnection, len(page.Items))
	for i, connection := range page.Items {
		items[i] = apiConnectionDTO(connection)
	}
	return gen.ListAPIConnections200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// CreateAPIConnection handles reusable API connection creation.
func (h *ToolingHandler) CreateAPIConnection(ctx context.Context, request gen.CreateAPIConnectionRequestObject) (gen.CreateAPIConnectionResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	command := inbound.APIConnectionCreateCommand{Slug: request.Body.Slug, DisplayName: request.Body.DisplayName, BaseURL: request.Body.BaseUrl}
	if request.Body.PublicHeaders != nil {
		command.PublicHeaders = *request.Body.PublicHeaders
	}
	if request.Body.SecretHeaders != nil {
		command.SecretHeaders = *request.Body.SecretHeaders
	}
	connection, err := h.connections.CreateAPIConnection(ctx, requestFrom(ctx).principal, command)
	if err != nil {
		return nil, err
	}
	return gen.CreateAPIConnection201JSONResponse(apiConnectionDTO(connection)), nil
}

// GetAPIConnection handles retrieval without exposing secret values.
func (h *ToolingHandler) GetAPIConnection(ctx context.Context, request gen.GetAPIConnectionRequestObject) (gen.GetAPIConnectionResponseObject, error) {
	connection, err := h.connections.GetAPIConnection(ctx, requestFrom(ctx).principal, request.ApiConnectionId)
	if err != nil {
		return nil, err
	}
	return gen.GetAPIConnection200JSONResponse(apiConnectionDTO(connection)), nil
}

// UpdateAPIConnection handles partial connection updates.
func (h *ToolingHandler) UpdateAPIConnection(ctx context.Context, request gen.UpdateAPIConnectionRequestObject) (gen.UpdateAPIConnectionResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	command := inbound.APIConnectionUpdateCommand{
		Slug: request.Body.Slug, DisplayName: request.Body.DisplayName, BaseURL: request.Body.BaseUrl,
		PublicHeaders: request.Body.PublicHeaders, SecretHeaders: changeValue(request.Body.SecretHeaders),
	}
	connection, err := h.connections.UpdateAPIConnection(ctx, requestFrom(ctx).principal, request.ApiConnectionId, command)
	if err != nil {
		return nil, err
	}
	return gen.UpdateAPIConnection200JSONResponse(apiConnectionDTO(connection)), nil
}

// DeleteAPIConnection protects referenced tools and deletes unused configuration.
func (h *ToolingHandler) DeleteAPIConnection(ctx context.Context, request gen.DeleteAPIConnectionRequestObject) (gen.DeleteAPIConnectionResponseObject, error) {
	err := h.connections.DeleteAPIConnection(ctx, requestFrom(ctx).principal, request.ApiConnectionId)
	return gen.DeleteAPIConnection204Response{}, err
}
