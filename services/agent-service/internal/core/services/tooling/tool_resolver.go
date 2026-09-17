package tooling

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// Resolve builds an isolated, lazily connected tool set for one agent run.
func (s *Service) Resolve(ctx context.Context, agentID uuid.UUID) (outbound.ToolSet, error) {
	catalog, err := s.Bindings.ResolveAgentTools(ctx, agentID)
	if err != nil {
		return nil, err
	}
	set := &resolvedToolSet{httpInvoker: s.HTTP, mcpConnector: s.MCP, entries: map[string]resolvedEntry{}, sessions: map[uuid.UUID]outbound.MCPSession{}}
	used := map[string]bool{}
	connections := map[uuid.UUID]apiConnectionSnapshot{}
	for _, connection := range catalog.APIConnections {
		connectionSecrets, decryptErr := s.decryptHeaders("api_connection", connection.ID, connection.SecretHeadersCiphertext)
		if decryptErr != nil {
			return nil, decryptErr
		}
		connections[connection.ID] = apiConnectionSnapshot{connection: connection, secrets: connectionSecrets}
	}
	for _, tool := range catalog.HTTPTools {
		resolvedTool, secrets, decryptErr := s.resolveHTTPTool(tool, connections)
		if decryptErr != nil {
			return nil, decryptErr
		}
		name := domain.UniqueToolName("http_", tool.Slug, "http:"+tool.ID.String(), used)
		set.specs = append(set.specs, domain.ToolSpec{Name: name, Ref: tool.Slug, Description: tool.Description, JSONSchema: domain.ToolJSONSchema(tool.Params)})
		set.entries[name] = resolvedEntry{httpTool: &resolvedTool, secrets: secrets}
	}
	for _, server := range catalog.MCPServers {
		secrets, decryptErr := s.decryptHeaders("mcp", server.ID, server.SecretHeadersCiphertext)
		if decryptErr != nil {
			return nil, decryptErr
		}
		allowed := allowedSet(server.AllowedTools)
		cachedTools := append([]domain.MCPTool(nil), server.Tools...)
		sort.SliceStable(cachedTools, func(i, j int) bool { return cachedTools[i].Name < cachedTools[j].Name })
		for _, tool := range cachedTools {
			if allowed != nil && !allowed[tool.Name] {
				continue
			}
			name := domain.UniqueToolName("mcp_"+server.Slug+"_", tool.Name, fmt.Sprintf("mcp:%s:%s", server.ID, tool.Name), used)
			set.specs = append(set.specs, domain.ToolSpec{Name: name, Ref: server.Slug + "." + tool.Name, Description: tool.Description, JSONSchema: append([]byte(nil), tool.InputSchema...)})
			serverCopy := server
			set.entries[name] = resolvedEntry{mcpServer: &serverCopy, mcpToolName: tool.Name, secrets: secrets}
		}
	}
	return set, nil
}

type apiConnectionSnapshot struct {
	connection domain.APIConnection
	secrets    map[string]string
}

func (s *Service) resolveHTTPTool(tool domain.HTTPTool, connections map[uuid.UUID]apiConnectionSnapshot) (domain.HTTPTool, map[string]string, error) {
	toolSecrets, err := s.decryptHeaders("tool", tool.ID, tool.SecretHeadersCiphertext)
	if err != nil {
		return domain.HTTPTool{}, nil, err
	}
	if tool.ConnectionID == nil {
		return tool, toolSecrets, nil
	}
	snapshot, ok := connections[*tool.ConnectionID]
	if !ok {
		return domain.HTTPTool{}, nil, domain.ErrNotFound
	}
	resolvedURL, err := domain.JoinAPIConnectionURL(snapshot.connection.BaseURL, tool.URLTemplate)
	if err != nil {
		return domain.HTTPTool{}, nil, err
	}
	if err = s.URLs.ValidateURL(resolvedURL); err != nil {
		return domain.HTTPTool{}, nil, domain.ErrEgressDenied
	}
	tool.ConnectionID = nil
	tool.URLTemplate = resolvedURL
	tool.PublicHeaders = mergeHeaders(snapshot.connection.PublicHeaders, tool.PublicHeaders)
	return tool, mergeHeaders(snapshot.secrets, toolSecrets), nil
}

func mergeHeaders(base, override map[string]string) map[string]string {
	result := cloneHeaders(base)
	for name, value := range override {
		for existing := range result {
			if strings.EqualFold(existing, name) {
				delete(result, existing)
			}
		}
		result[name] = value
	}
	return result
}

func allowedSet(value *[]string) map[string]bool {
	if value == nil {
		return nil
	}
	result := make(map[string]bool, len(*value))
	for _, name := range *value {
		result[name] = true
	}
	return result
}

var _ outbound.ToolResolver = (*Service)(nil)
