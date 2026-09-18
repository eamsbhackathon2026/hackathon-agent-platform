package postgres

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

var _ outbound.AgentToolBindingRepository = (*Store)(nil)

// GetAgentToolBindings returns all HTTP and MCP sources enabled for an agent.
func (s *Store) GetAgentToolBindings(ctx context.Context, agentID uuid.UUID) (domain.AgentToolBindings, error) {
	toolIDs, err := s.queries(ctx).ListAgentToolIDs(ctx, dbID(agentID))
	if err != nil {
		return domain.AgentToolBindings{}, mapError(err)
	}
	serverIDs, err := s.queries(ctx).ListAgentMCPServerIDs(ctx, dbID(agentID))
	if err != nil {
		return domain.AgentToolBindings{}, mapError(err)
	}
	result := domain.AgentToolBindings{ToolIDs: make([]uuid.UUID, len(toolIDs)), MCPServerIDs: make([]uuid.UUID, len(serverIDs))}
	for i, id := range toolIDs {
		result.ToolIDs[i] = uuid.UUID(id.Bytes)
	}
	for i, id := range serverIDs {
		result.MCPServerIDs[i] = uuid.UUID(id.Bytes)
	}
	return result, nil
}

// ReplaceAgentToolBindings atomically replaces all tool bindings in the active transaction.
func (s *Store) ReplaceAgentToolBindings(ctx context.Context, agentID uuid.UUID, bindings domain.AgentToolBindings) error {
	queries := s.queries(ctx)
	if err := queries.DeleteAgentTools(ctx, dbID(agentID)); err != nil {
		return mapError(err)
	}
	if err := queries.DeleteAgentMCPServers(ctx, dbID(agentID)); err != nil {
		return mapError(err)
	}
	for _, id := range bindings.ToolIDs {
		if err := queries.AddAgentTool(ctx, sqlcgen.AddAgentToolParams{AgentID: dbID(agentID), ToolID: dbID(id)}); err != nil {
			return mapError(err)
		}
	}
	for _, id := range bindings.MCPServerIDs {
		if err := queries.AddAgentMCPServer(ctx, sqlcgen.AddAgentMCPServerParams{AgentID: dbID(agentID), McpServerID: dbID(id)}); err != nil {
			return mapError(err)
		}
	}
	return nil
}

// ResolveAgentTools loads HTTP tools and their API connections from one SQL
// statement so each run observes a coherent connection snapshot without N+1 reads.
func (s *Store) ResolveAgentTools(ctx context.Context, agentID uuid.UUID) (outbound.AgentToolCatalog, error) {
	toolRows, err := s.queries(ctx).ResolveAgentHTTPTools(ctx, dbID(agentID))
	if err != nil {
		return outbound.AgentToolCatalog{}, mapError(err)
	}
	catalog := outbound.AgentToolCatalog{HTTPTools: make([]domain.HTTPTool, 0, len(toolRows))}
	seenConnections := make(map[uuid.UUID]bool)
	tools := make([]domain.HTTPTool, 0, len(toolRows))
	for _, row := range toolRows {
		tool, connection, modelErr := resolvedHTTPToolModels(row)
		if modelErr != nil {
			return outbound.AgentToolCatalog{}, modelErr
		}
		tools = append(tools, tool)
		if connection != nil && !seenConnections[connection.ID] {
			catalog.APIConnections = append(catalog.APIConnections, *connection)
			seenConnections[connection.ID] = true
		}
	}
	catalog.HTTPTools = tools
	serverRows, err := s.queries(ctx).ResolveAgentMCPServers(ctx, dbID(agentID))
	if err != nil {
		return outbound.AgentToolCatalog{}, mapError(err)
	}
	servers := make([]domain.MCPServer, 0, len(serverRows))
	for _, row := range serverRows {
		server, modelErr := mcpServerModel(row)
		if modelErr != nil {
			return outbound.AgentToolCatalog{}, modelErr
		}
		servers = append(servers, server)
	}
	catalog.MCPServers = servers
	return catalog, nil
}

func resolvedHTTPToolModels(row sqlcgen.ResolveAgentHTTPToolsRow) (domain.HTTPTool, *domain.APIConnection, error) {
	tool, err := httpToolModel(sqlcgen.Tool{
		ID: row.ID, Slug: row.Slug, DisplayName: row.DisplayName, StepLabel: row.StepLabel, Description: row.Description,
		Kind: row.Kind, Method: row.Method, UrlTemplate: row.UrlTemplate, Params: row.Params,
		PublicHeaders: row.PublicHeaders, SecretHeadersCiphertext: row.SecretHeadersCiphertext,
		SecretHeaderNames: row.SecretHeaderNames, TimeoutSeconds: row.TimeoutSeconds,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ConnectionID: row.ConnectionID,
	})
	if err != nil || tool.ConnectionID == nil {
		return tool, nil, err
	}
	if !row.ApiConnectionID.Valid {
		return domain.HTTPTool{}, nil, domain.ErrNotFound
	}
	headers, err := decodeStringMap(row.ApiConnectionPublicHeaders)
	if err != nil {
		return domain.HTTPTool{}, nil, err
	}
	connection := &domain.APIConnection{
		ID: uuid.UUID(row.ApiConnectionID.Bytes), Slug: row.ApiConnectionSlug.String,
		DisplayName: row.ApiConnectionDisplayName.String, BaseURL: row.ApiConnectionBaseUrl.String,
		PublicHeaders: headers, SecretHeadersCiphertext: append([]byte(nil), row.ApiConnectionSecretHeadersCiphertext...),
		SecretHeaderNames: append([]string{}, row.ApiConnectionSecretHeaderNames...),
		CreatedAt:         row.ApiConnectionCreatedAt.Time.UTC(), UpdatedAt: row.ApiConnectionUpdatedAt.Time.UTC(),
	}
	return tool, connection, nil
}
