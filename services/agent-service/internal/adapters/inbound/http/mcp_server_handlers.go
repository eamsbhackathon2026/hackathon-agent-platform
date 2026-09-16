package http

import (
	"context"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// ListMcpServers handles cursor-paginated MCP endpoint retrieval.
func (h *ToolingHandler) ListMcpServers(ctx context.Context, request gen.ListMcpServersRequestObject) (gen.ListMcpServersResponseObject, error) {
	page, err := h.servers.ListMCPServers(ctx, requestFrom(ctx).principal, pageRequest(request.Params.Limit, request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.McpServer, len(page.Items))
	for i, server := range page.Items {
		items[i] = mcpServerDTO(server)
	}
	return gen.ListMcpServers200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// CreateMcpServer handles creation of one MCP endpoint.
func (h *ToolingHandler) CreateMcpServer(ctx context.Context, request gen.CreateMcpServerRequestObject) (gen.CreateMcpServerResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	command := inbound.MCPServerCreateCommand{Slug: request.Body.Slug, DisplayName: request.Body.DisplayName, URL: request.Body.Url, AllowedTools: valuePointer(request.Body.AllowedTools)}
	if request.Body.SecretHeaders != nil {
		command.SecretHeaders = *request.Body.SecretHeaders
	}
	server, err := h.servers.CreateMCPServer(ctx, requestFrom(ctx).principal, command)
	if err != nil {
		return nil, err
	}
	return gen.CreateMcpServer201JSONResponse(mcpServerDTO(server)), nil
}

// GetMcpServer handles retrieval of one MCP endpoint.
func (h *ToolingHandler) GetMcpServer(ctx context.Context, request gen.GetMcpServerRequestObject) (gen.GetMcpServerResponseObject, error) {
	server, err := h.servers.GetMCPServer(ctx, requestFrom(ctx).principal, request.ServerId)
	if err != nil {
		return nil, err
	}
	return gen.GetMcpServer200JSONResponse(mcpServerDTO(server)), nil
}

// UpdateMcpServer handles a partial MCP endpoint update.
func (h *ToolingHandler) UpdateMcpServer(ctx context.Context, request gen.UpdateMcpServerRequestObject) (gen.UpdateMcpServerResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	command := inbound.MCPServerUpdateCommand{Slug: request.Body.Slug, DisplayName: request.Body.DisplayName, URL: request.Body.Url, AllowedTools: changeValue(request.Body.AllowedTools), SecretHeaders: changeValue(request.Body.SecretHeaders)}
	server, err := h.servers.UpdateMCPServer(ctx, requestFrom(ctx).principal, request.ServerId, command)
	if err != nil {
		return nil, err
	}
	return gen.UpdateMcpServer200JSONResponse(mcpServerDTO(server)), nil
}

// DeleteMcpServer handles removal of one MCP endpoint.
func (h *ToolingHandler) DeleteMcpServer(ctx context.Context, request gen.DeleteMcpServerRequestObject) (gen.DeleteMcpServerResponseObject, error) {
	err := h.servers.DeleteMCPServer(ctx, requestFrom(ctx).principal, request.ServerId)
	return gen.DeleteMcpServer204Response{}, err
}

// RefreshMcpServer handles discovery and caching of remote MCP tools.
func (h *ToolingHandler) RefreshMcpServer(ctx context.Context, request gen.RefreshMcpServerRequestObject) (gen.RefreshMcpServerResponseObject, error) {
	server, err := h.servers.RefreshMCPServer(ctx, requestFrom(ctx).principal, request.ServerId)
	if err != nil {
		return nil, err
	}
	return gen.RefreshMcpServer200JSONResponse(mcpServerDTO(server)), nil
}
