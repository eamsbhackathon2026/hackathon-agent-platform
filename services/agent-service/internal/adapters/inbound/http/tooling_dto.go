package http

import (
	"encoding/json"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

func toolParams(values []gen.ToolParam) []domain.ToolParam {
	result := make([]domain.ToolParam, len(values))
	for i, value := range values {
		param := domain.ToolParam{Name: value.Name, Description: value.Description, Type: domain.ToolParamType(value.Type), Required: value.Required, Location: domain.ToolParamLocation(value.In)}
		if value.Fields != nil {
			param.Fields = make([]domain.ToolParamField, len(*value.Fields))
			for j, field := range *value.Fields {
				param.Fields[j] = domain.ToolParamField{Name: field.Name, Description: field.Description, Type: domain.ToolParamType(field.Type), Required: field.Required}
			}
		}
		if value.ItemType != nil {
			param.ItemType = domain.ToolParamType(*value.ItemType)
		}
		result[i] = param
	}
	return result
}

func toolParamsDTO(values []domain.ToolParam) []gen.ToolParam {
	result := make([]gen.ToolParam, len(values))
	for i, value := range values {
		param := gen.ToolParam{Name: value.Name, Description: value.Description, Type: gen.ToolParamType(value.Type), Required: value.Required, In: gen.ToolParamIn(value.Location)}
		if len(value.Fields) > 0 {
			fields := make([]gen.ToolParamField, len(value.Fields))
			for j, field := range value.Fields {
				fields[j] = gen.ToolParamField{Name: field.Name, Description: field.Description, Type: gen.ToolParamFieldType(field.Type), Required: field.Required}
			}
			param.Fields = &fields
		}
		if value.ItemType != "" {
			itemType := gen.ToolParamItemType(value.ItemType)
			param.ItemType = &itemType
		}
		result[i] = param
	}
	return result
}

func httpToolDTO(tool domain.HTTPTool) gen.HttpTool {
	return gen.HttpTool{Id: tool.ID, ConnectionId: nullableValue(tool.ConnectionID), Slug: tool.Slug, DisplayName: tool.DisplayName, Description: tool.Description, Kind: gen.Http, Method: gen.HttpToolMethod(tool.Method), UrlTemplate: tool.URLTemplate, Params: toolParamsDTO(tool.Params), PublicHeaders: tool.PublicHeaders, SecretHeaderNames: append([]string{}, tool.SecretHeaderNames...), TimeoutSeconds: tool.TimeoutSeconds, CreatedAt: tool.CreatedAt, UpdatedAt: tool.UpdatedAt}
}

func apiConnectionDTO(connection domain.APIConnection) gen.ApiConnection {
	return gen.ApiConnection{Id: connection.ID, Slug: connection.Slug, DisplayName: connection.DisplayName, BaseUrl: connection.BaseURL, PublicHeaders: connection.PublicHeaders, SecretHeaderNames: append([]string{}, connection.SecretHeaderNames...), CreatedAt: connection.CreatedAt, UpdatedAt: connection.UpdatedAt}
}

func mcpServerDTO(server domain.MCPServer) gen.McpServer {
	tools := make([]gen.McpTool, len(server.Tools))
	for i, tool := range server.Tools {
		schema := map[string]any{}
		_ = json.Unmarshal(tool.InputSchema, &schema)
		tools[i] = gen.McpTool{Name: tool.Name, Description: tool.Description, InputSchema: schema}
	}
	return gen.McpServer{Id: server.ID, Slug: server.Slug, DisplayName: server.DisplayName, Url: server.URL, AllowedTools: nullableValue(server.AllowedTools), SecretHeaderNames: append([]string{}, server.SecretHeaderNames...), Tools: tools, Status: gen.ConnectionStatus(server.Status), LastError: failureValue(server.LastError), LastSyncedAt: nullableValue(server.LastSyncedAt), CreatedAt: server.CreatedAt, UpdatedAt: server.UpdatedAt}
}

func bindingsDTO(bindings domain.AgentToolBindings) gen.AgentToolBindings {
	return gen.AgentToolBindings{ToolIds: bindings.ToolIDs, McpServerIds: bindings.MCPServerIDs}
}

func toolTestDTO(result inbound.ToolTestResult) gen.ToolTestResult {
	duration := result.Duration.Milliseconds()
	if duration < 0 {
		duration = 0
	}
	return gen.ToolTestResult{Ok: result.OK, StatusCode: nullableValue(result.StatusCode), Body: result.Body, Truncated: result.Truncated, DurationMs: duration, Error: failureValue(result.Failure)}
}
