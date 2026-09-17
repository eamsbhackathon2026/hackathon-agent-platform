package http

import (
	"context"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
)

// ExportTools handles download of the portable HTTP tool bundle.
func (h *ToolingHandler) ExportTools(ctx context.Context, _ gen.ExportToolsRequestObject) (gen.ExportToolsResponseObject, error) {
	bundle, err := h.transfers.ExportTools(ctx, requestFrom(ctx).principal)
	if err != nil {
		return nil, err
	}
	return gen.ExportTools200JSONResponse(toolBundleDTO(bundle)), nil
}

// PreviewToolImport handles the read-only classification of an uploaded bundle.
func (h *ToolingHandler) PreviewToolImport(ctx context.Context, request gen.PreviewToolImportRequestObject) (gen.PreviewToolImportResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	items, err := h.transfers.PreviewToolImport(ctx, requestFrom(ctx).principal, toolBundle(gen.ToolBundle(*request.Body)))
	if err != nil {
		return nil, err
	}
	response := gen.PreviewToolImport200JSONResponse{Items: make([]gen.ToolImportItem, len(items))}
	for i, item := range items {
		fields := make([]gen.FieldError, len(item.Fields))
		for j, field := range item.Fields {
			fields[j] = gen.FieldError{Field: field.Field, Message: field.Message}
		}
		response.Items[i] = gen.ToolImportItem{Kind: gen.ToolImportKind(item.Kind), Slug: item.Slug, DisplayName: item.DisplayName, Status: gen.ToolImportItemStatus(item.Status), Fields: fields}
	}
	return response, nil
}

// ImportTools handles the transactional import of a bundle with per-entry decisions.
func (h *ToolingHandler) ImportTools(ctx context.Context, request gen.ImportToolsRequestObject) (gen.ImportToolsResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	decisions := make(map[domain.ToolImportKey]domain.ToolImportAction, len(request.Body.Decisions))
	for _, decision := range request.Body.Decisions {
		decisions[domain.ToolImportKey{Kind: domain.ToolImportKind(decision.Kind), Slug: decision.Slug}] = domain.ToolImportAction(decision.Action)
	}
	result, err := h.transfers.ImportTools(ctx, requestFrom(ctx).principal, toolBundle(request.Body.Bundle), decisions)
	if err != nil {
		return nil, err
	}
	response := gen.ImportTools200JSONResponse{Created: result.Created, Overwritten: result.Overwritten, Skipped: result.Skipped, NeedsSecrets: make([]gen.ToolImportSecretReminder, len(result.NeedsSecrets))}
	for i, reminder := range result.NeedsSecrets {
		response.NeedsSecrets[i] = gen.ToolImportSecretReminder{Kind: gen.ToolImportKind(reminder.Kind), Id: reminder.ID, Slug: reminder.Slug, DisplayName: reminder.DisplayName, HeaderNames: append([]string{}, reminder.HeaderNames...)}
	}
	return response, nil
}

func toolBundle(value gen.ToolBundle) domain.ToolBundle {
	bundle := domain.ToolBundle{Format: string(value.Format), Version: int(value.Version), Connections: make([]domain.ToolBundleConnection, len(value.Connections)), Tools: make([]domain.ToolBundleTool, len(value.Tools))}
	if value.ExportedAt != nil {
		bundle.ExportedAt = *value.ExportedAt
	}
	for i, connection := range value.Connections {
		bundle.Connections[i] = domain.ToolBundleConnection{Slug: connection.Slug, DisplayName: connection.DisplayName, BaseURL: connection.BaseUrl, PublicHeaders: connection.PublicHeaders, SecretHeaderNames: connection.SecretHeaderNames}
	}
	for i, tool := range value.Tools {
		bundle.Tools[i] = domain.ToolBundleTool{Slug: tool.Slug, DisplayName: tool.DisplayName, Description: tool.Description, Method: domain.HTTPToolMethod(tool.Method), URLTemplate: tool.UrlTemplate, ConnectionSlug: valuePointer(tool.ConnectionSlug), Params: toolParams(tool.Params), PublicHeaders: tool.PublicHeaders, TimeoutSeconds: tool.TimeoutSeconds, SecretHeaderNames: tool.SecretHeaderNames}
	}
	return bundle
}

func toolBundleDTO(bundle domain.ToolBundle) gen.ToolBundle {
	exportedAt := bundle.ExportedAt
	value := gen.ToolBundle{Format: gen.ToolBundleFormat(bundle.Format), Version: gen.ToolBundleVersion(bundle.Version), ExportedAt: &exportedAt, Connections: make([]gen.ToolBundleConnection, len(bundle.Connections)), Tools: make([]gen.ToolBundleTool, len(bundle.Tools))}
	for i, connection := range bundle.Connections {
		value.Connections[i] = gen.ToolBundleConnection{Slug: connection.Slug, DisplayName: connection.DisplayName, BaseUrl: connection.BaseURL, PublicHeaders: nonNilHeaders(connection.PublicHeaders), SecretHeaderNames: append([]string{}, connection.SecretHeaderNames...)}
	}
	for i, tool := range bundle.Tools {
		value.Tools[i] = gen.ToolBundleTool{Slug: tool.Slug, DisplayName: tool.DisplayName, Description: tool.Description, Method: gen.ToolBundleToolMethod(tool.Method), UrlTemplate: tool.URLTemplate, ConnectionSlug: nullableValue(tool.ConnectionSlug), Params: toolParamsDTO(tool.Params), PublicHeaders: nonNilHeaders(tool.PublicHeaders), TimeoutSeconds: tool.TimeoutSeconds, SecretHeaderNames: append([]string{}, tool.SecretHeaderNames...)}
	}
	return value
}

// nonNilHeaders keeps an empty header set as {} in JSON, which the bundle schema requires.
func nonNilHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return map[string]string{}
	}
	return headers
}
