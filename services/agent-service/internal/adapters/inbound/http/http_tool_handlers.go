package http

import (
	"context"
	"encoding/json"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// ListTools handles cursor-paginated HTTP tool retrieval.
func (h *ToolingHandler) ListTools(ctx context.Context, request gen.ListToolsRequestObject) (gen.ListToolsResponseObject, error) {
	page, err := h.tools.ListTools(ctx, requestFrom(ctx).principal, pageRequest(request.Params.Limit, request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.HttpTool, len(page.Items))
	for i, tool := range page.Items {
		items[i] = httpToolDTO(tool)
	}
	return gen.ListTools200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// CreateTool handles creation of one saved HTTP tool.
func (h *ToolingHandler) CreateTool(ctx context.Context, request gen.CreateToolRequestObject) (gen.CreateToolResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	timeout := 15
	if request.Body.TimeoutSeconds != nil {
		timeout = *request.Body.TimeoutSeconds
	}
	command := inbound.HTTPToolCreateCommand{ConnectionID: valuePointer(request.Body.ConnectionId), Slug: request.Body.Slug, DisplayName: request.Body.DisplayName, Description: stringValue(request.Body.Description), Method: domain.HTTPToolMethod(request.Body.Method), URLTemplate: request.Body.UrlTemplate, Params: toolParams(request.Body.Params), TimeoutSeconds: timeout}
	if request.Body.PublicHeaders != nil {
		command.PublicHeaders = *request.Body.PublicHeaders
	}
	if request.Body.SecretHeaders != nil {
		command.SecretHeaders = *request.Body.SecretHeaders
	}
	tool, err := h.tools.CreateTool(ctx, requestFrom(ctx).principal, command)
	if err != nil {
		return nil, err
	}
	return gen.CreateTool201JSONResponse(httpToolDTO(tool)), nil
}

// GetTool handles retrieval of one saved HTTP tool.
func (h *ToolingHandler) GetTool(ctx context.Context, request gen.GetToolRequestObject) (gen.GetToolResponseObject, error) {
	tool, err := h.tools.GetTool(ctx, requestFrom(ctx).principal, request.ToolId)
	if err != nil {
		return nil, err
	}
	return gen.GetTool200JSONResponse(httpToolDTO(tool)), nil
}

// UpdateTool handles a partial saved HTTP tool update.
func (h *ToolingHandler) UpdateTool(ctx context.Context, request gen.UpdateToolRequestObject) (gen.UpdateToolResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	command := inbound.HTTPToolUpdateCommand{ConnectionID: changeValue(request.Body.ConnectionId), Slug: request.Body.Slug, DisplayName: request.Body.DisplayName, Description: request.Body.Description, URLTemplate: request.Body.UrlTemplate, PublicHeaders: request.Body.PublicHeaders, TimeoutSeconds: request.Body.TimeoutSeconds, SecretHeaders: changeValue(request.Body.SecretHeaders)}
	if request.Body.Method != nil {
		method := domain.HTTPToolMethod(*request.Body.Method)
		command.Method = &method
	}
	if request.Body.Params != nil {
		params := toolParams(*request.Body.Params)
		command.Params = &params
	}
	tool, err := h.tools.UpdateTool(ctx, requestFrom(ctx).principal, request.ToolId, command)
	if err != nil {
		return nil, err
	}
	return gen.UpdateTool200JSONResponse(httpToolDTO(tool)), nil
}

// DeleteTool handles removal of one saved HTTP tool.
func (h *ToolingHandler) DeleteTool(ctx context.Context, request gen.DeleteToolRequestObject) (gen.DeleteToolResponseObject, error) {
	err := h.tools.DeleteTool(ctx, requestFrom(ctx).principal, request.ToolId)
	return gen.DeleteTool204Response{}, err
}

// TestTool handles a manual saved HTTP tool execution.
func (h *ToolingHandler) TestTool(ctx context.Context, request gen.TestToolRequestObject) (gen.TestToolResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	args, err := json.Marshal(request.Body.Args)
	if err != nil {
		return nil, domain.ErrValidation
	}
	result, err := h.tools.TestTool(ctx, requestFrom(ctx).principal, request.ToolId, args)
	if err != nil {
		return nil, err
	}
	return gen.TestTool200JSONResponse(toolTestDTO(result)), nil
}
