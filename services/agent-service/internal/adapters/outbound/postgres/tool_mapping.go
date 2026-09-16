package postgres

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
)

type toolParamJSON struct {
	Name        string               `json:"name"`
	Type        string               `json:"type"`
	Description string               `json:"description"`
	Required    bool                 `json:"required"`
	Location    string               `json:"in"`
	Fields      []toolParamFieldJSON `json:"fields,omitempty"`
	ItemType    string               `json:"item_type,omitempty"`
}

type toolParamFieldJSON struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type mcpToolJSON struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func encodeToolParams(params []domain.ToolParam) []byte {
	values := make([]toolParamJSON, len(params))
	for i, param := range params {
		fields := make([]toolParamFieldJSON, len(param.Fields))
		for j, field := range param.Fields {
			fields[j] = toolParamFieldJSON{Name: field.Name, Type: string(field.Type), Description: field.Description, Required: field.Required}
		}
		values[i] = toolParamJSON{Name: param.Name, Type: string(param.Type), Description: param.Description, Required: param.Required, Location: string(param.Location), Fields: fields, ItemType: string(param.ItemType)}
	}
	value, _ := json.Marshal(values)
	return value
}

func decodeToolParams(value []byte) ([]domain.ToolParam, error) {
	var values []toolParamJSON
	if err := json.Unmarshal(value, &values); err != nil {
		return nil, fmt.Errorf("decode tool params: %w", err)
	}
	result := make([]domain.ToolParam, len(values))
	for i, param := range values {
		fields := make([]domain.ToolParamField, len(param.Fields))
		for j, field := range param.Fields {
			fields[j] = domain.ToolParamField{Name: field.Name, Type: domain.ToolParamType(field.Type), Description: field.Description, Required: field.Required}
		}
		result[i] = domain.ToolParam{Name: param.Name, Type: domain.ToolParamType(param.Type), Description: param.Description, Required: param.Required, Location: domain.ToolParamLocation(param.Location), Fields: fields, ItemType: domain.ToolParamType(param.ItemType)}
	}
	return result, nil
}

func encodeStringMap(value map[string]string) []byte {
	if value == nil {
		value = map[string]string{}
	}
	encoded, _ := json.Marshal(value)
	return encoded
}

func decodeStringMap(value []byte) (map[string]string, error) {
	result := map[string]string{}
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, fmt.Errorf("decode tool headers: %w", err)
	}
	return result, nil
}

func encodeMCPTools(tools []domain.MCPTool) ([]byte, error) {
	values := make([]mcpToolJSON, len(tools))
	for i, tool := range tools {
		if !json.Valid(tool.InputSchema) {
			return nil, domain.ErrValidation
		}
		values[i] = mcpToolJSON{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema}
	}
	return json.Marshal(values)
}

func decodeMCPTools(value []byte) ([]domain.MCPTool, error) {
	var values []mcpToolJSON
	if err := json.Unmarshal(value, &values); err != nil {
		return nil, fmt.Errorf("decode MCP tools: %w", err)
	}
	result := make([]domain.MCPTool, len(values))
	for i, tool := range values {
		result[i] = domain.MCPTool{Name: tool.Name, Description: tool.Description, InputSchema: append(json.RawMessage(nil), tool.InputSchema...)}
	}
	return result, nil
}

func httpToolModel(v sqlcgen.Tool) (domain.HTTPTool, error) {
	params, err := decodeToolParams(v.Params)
	if err != nil {
		return domain.HTTPTool{}, err
	}
	headers, err := decodeStringMap(v.PublicHeaders)
	if err != nil {
		return domain.HTTPTool{}, err
	}
	return domain.HTTPTool{ID: uuid.UUID(v.ID.Bytes), ConnectionID: idPointer(v.ConnectionID), Slug: v.Slug, DisplayName: v.DisplayName, Description: v.Description, Method: domain.HTTPToolMethod(v.Method), URLTemplate: v.UrlTemplate, Params: params, PublicHeaders: headers, SecretHeadersCiphertext: append([]byte(nil), v.SecretHeadersCiphertext...), SecretHeaderNames: append([]string(nil), v.SecretHeaderNames...), TimeoutSeconds: int(v.TimeoutSeconds), CreatedAt: v.CreatedAt.Time.UTC(), UpdatedAt: v.UpdatedAt.Time.UTC()}, nil
}

func mcpServerModel(v sqlcgen.McpServer) (domain.MCPServer, error) {
	tools, err := decodeMCPTools(v.ToolsCache)
	if err != nil {
		return domain.MCPServer{}, err
	}
	server := domain.MCPServer{ID: uuid.UUID(v.ID.Bytes), Slug: v.Slug, DisplayName: v.DisplayName, URL: v.Url, SecretHeadersCiphertext: append([]byte(nil), v.SecretHeadersCiphertext...), SecretHeaderNames: append([]string(nil), v.SecretHeaderNames...), Tools: tools, Status: domain.ConnectionStatus(v.Status), LastSyncedAt: catalogTimePointer(v.LastSyncedAt), CreatedAt: v.CreatedAt.Time.UTC(), UpdatedAt: v.UpdatedAt.Time.UTC(), Revision: v.Revision}
	if v.AllowedTools != nil {
		allowed := append([]string(nil), v.AllowedTools...)
		server.AllowedTools = &allowed
	}
	if len(v.LastError) > 0 {
		var failure providerFailureJSON
		if err = json.Unmarshal(v.LastError, &failure); err != nil {
			return domain.MCPServer{}, fmt.Errorf("decode MCP failure: %w", err)
		}
		server.LastError = &domain.ProviderFailure{Code: failure.Code, Message: failure.Message}
	}
	return server, nil
}
