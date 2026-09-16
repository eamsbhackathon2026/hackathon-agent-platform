package fakes

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

func cloneHTTPTool(tool domain.HTTPTool) domain.HTTPTool {
	tool.ConnectionID = copyPointer(tool.ConnectionID)
	tool.Params = append([]domain.ToolParam{}, tool.Params...)
	tool.PublicHeaders = cloneStringMap(tool.PublicHeaders)
	tool.SecretHeadersCiphertext = append([]byte(nil), tool.SecretHeadersCiphertext...)
	tool.SecretHeaderNames = append([]string{}, tool.SecretHeaderNames...)
	return tool
}

func cloneAPIConnection(connection domain.APIConnection) domain.APIConnection {
	connection.PublicHeaders = cloneStringMap(connection.PublicHeaders)
	connection.SecretHeadersCiphertext = append([]byte(nil), connection.SecretHeadersCiphertext...)
	connection.SecretHeaderNames = append([]string{}, connection.SecretHeaderNames...)
	return connection
}

func cloneMCPServer(server domain.MCPServer) domain.MCPServer {
	server.AllowedTools = copySlicePointer(server.AllowedTools)
	server.SecretHeadersCiphertext = append([]byte(nil), server.SecretHeadersCiphertext...)
	server.SecretHeaderNames = append([]string{}, server.SecretHeaderNames...)
	server.Tools = append([]domain.MCPTool{}, server.Tools...)
	for i := range server.Tools {
		server.Tools[i].InputSchema = append([]byte(nil), server.Tools[i].InputSchema...)
	}
	server.LastError = copyPointer(server.LastError)
	server.LastSyncedAt = copyPointer(server.LastSyncedAt)
	return server
}

func cloneBindings(bindings domain.AgentToolBindings) domain.AgentToolBindings {
	bindings.ToolIDs = append([]uuid.UUID{}, bindings.ToolIDs...)
	bindings.MCPServerIDs = append([]uuid.UUID{}, bindings.MCPServerIDs...)
	return bindings
}

func cloneStringMap(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func copySlicePointer[T any](value *[]T) *[]T {
	if value == nil {
		return nil
	}
	result := append([]T{}, (*value)...)
	return &result
}

// LockTooling emulates the repository-wide mutation lock.
func (s *CatalogStore) LockTooling(ctx context.Context) error { defer s.lock(ctx)(); return nil }

// CreateAPIConnection inserts reusable API configuration into the fake store.
func (s *CatalogStore) CreateAPIConnection(ctx context.Context, connection domain.APIConnection) error {
	defer s.lock(ctx)()
	for _, existing := range s.connections {
		if existing.ID == connection.ID || existing.Slug == connection.Slug {
			return domain.ErrConflict
		}
	}
	s.connections[connection.ID] = cloneAPIConnection(connection)
	return nil
}

// GetAPIConnection returns reusable API configuration from the fake store.
func (s *CatalogStore) GetAPIConnection(ctx context.Context, id uuid.UUID) (domain.APIConnection, error) {
	defer s.lock(ctx)()
	connection, ok := s.connections[id]
	if !ok {
		return domain.APIConnection{}, domain.ErrNotFound
	}
	return cloneAPIConnection(connection), nil
}

// GetAPIConnectionForUpdate returns a connection for a fake transaction update.
func (s *CatalogStore) GetAPIConnectionForUpdate(ctx context.Context, id uuid.UUID) (domain.APIConnection, error) {
	return s.GetAPIConnection(ctx, id)
}

// ListAPIConnections returns keyset-paginated reusable connections.
func (s *CatalogStore) ListAPIConnections(ctx context.Context, options domain.PageOptions) ([]domain.APIConnection, error) {
	defer s.lock(ctx)()
	items := []domain.APIConnection{}
	for _, connection := range s.connections {
		if before(connection.CreatedAt, connection.ID, options) {
			items = append(items, cloneAPIConnection(connection))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, nil
}

// UpdateAPIConnection replaces reusable configuration in the fake store.
func (s *CatalogStore) UpdateAPIConnection(ctx context.Context, connection domain.APIConnection) error {
	defer s.lock(ctx)()
	if _, ok := s.connections[connection.ID]; !ok {
		return domain.ErrNotFound
	}
	for _, existing := range s.connections {
		if existing.ID != connection.ID && existing.Slug == connection.Slug {
			return domain.ErrConflict
		}
	}
	s.connections[connection.ID] = cloneAPIConnection(connection)
	return nil
}

// DeleteAPIConnection removes unused reusable configuration.
func (s *CatalogStore) DeleteAPIConnection(ctx context.Context, id uuid.UUID) error {
	defer s.lock(ctx)()
	if _, ok := s.connections[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.connections, id)
	return nil
}

// CountToolsByAPIConnection returns how many fake tools reference a connection.
func (s *CatalogStore) CountToolsByAPIConnection(ctx context.Context, id uuid.UUID) (int64, error) {
	defer s.lock(ctx)()
	var count int64
	for _, tool := range s.tools {
		if tool.ConnectionID != nil && *tool.ConnectionID == id {
			count++
		}
	}
	return count, nil
}

// CreateTool inserts one HTTP tool into the fake store.
func (s *CatalogStore) CreateTool(ctx context.Context, tool domain.HTTPTool) error {
	defer s.lock(ctx)()
	if tool.ConnectionID != nil {
		if _, ok := s.connections[*tool.ConnectionID]; !ok {
			return domain.ErrNotFound
		}
	}
	for _, existing := range s.tools {
		if existing.ID == tool.ID || existing.Slug == tool.Slug {
			return domain.ErrConflict
		}
	}
	s.tools[tool.ID] = cloneHTTPTool(tool)
	return nil
}

// GetTool returns one HTTP tool from the fake store.
func (s *CatalogStore) GetTool(ctx context.Context, id uuid.UUID) (domain.HTTPTool, error) {
	defer s.lock(ctx)()
	tool, ok := s.tools[id]
	if !ok {
		return domain.HTTPTool{}, domain.ErrNotFound
	}
	return cloneHTTPTool(tool), nil
}

// GetToolForUpdate returns one HTTP tool for a fake transaction update.
func (s *CatalogStore) GetToolForUpdate(ctx context.Context, id uuid.UUID) (domain.HTTPTool, error) {
	return s.GetTool(ctx, id)
}

// ListTools returns a keyset-paginated collection of fake HTTP tools.
func (s *CatalogStore) ListTools(ctx context.Context, options domain.PageOptions) ([]domain.HTTPTool, error) {
	defer s.lock(ctx)()
	items := []domain.HTTPTool{}
	for _, tool := range s.tools {
		if before(tool.CreatedAt, tool.ID, options) {
			items = append(items, cloneHTTPTool(tool))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, nil
}

// UpdateTool replaces one HTTP tool in the fake store.
func (s *CatalogStore) UpdateTool(ctx context.Context, tool domain.HTTPTool) error {
	defer s.lock(ctx)()
	if _, ok := s.tools[tool.ID]; !ok {
		return domain.ErrNotFound
	}
	if tool.ConnectionID != nil {
		if _, ok := s.connections[*tool.ConnectionID]; !ok {
			return domain.ErrNotFound
		}
	}
	for _, existing := range s.tools {
		if existing.ID != tool.ID && existing.Slug == tool.Slug {
			return domain.ErrConflict
		}
	}
	s.tools[tool.ID] = cloneHTTPTool(tool)
	return nil
}

// DeleteTool removes one HTTP tool and its fake bindings.
func (s *CatalogStore) DeleteTool(ctx context.Context, id uuid.UUID) error {
	defer s.lock(ctx)()
	if _, ok := s.tools[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.tools, id)
	for agentID, bindings := range s.bindings {
		filtered := bindings.ToolIDs[:0]
		for _, toolID := range bindings.ToolIDs {
			if toolID != id {
				filtered = append(filtered, toolID)
			}
		}
		bindings.ToolIDs = filtered
		s.bindings[agentID] = bindings
	}
	return nil
}

// CreateMCPServer inserts one MCP endpoint into the fake store.
func (s *CatalogStore) CreateMCPServer(ctx context.Context, server domain.MCPServer) error {
	defer s.lock(ctx)()
	for _, existing := range s.servers {
		if existing.ID == server.ID || existing.Slug == server.Slug {
			return domain.ErrConflict
		}
	}
	if server.Revision == 0 {
		server.Revision = 1
	}
	s.servers[server.ID] = cloneMCPServer(server)
	return nil
}

// GetMCPServer returns one MCP endpoint from the fake store.
func (s *CatalogStore) GetMCPServer(ctx context.Context, id uuid.UUID) (domain.MCPServer, error) {
	defer s.lock(ctx)()
	server, ok := s.servers[id]
	if !ok {
		return domain.MCPServer{}, domain.ErrNotFound
	}
	return cloneMCPServer(server), nil
}

// GetMCPServerForUpdate returns one endpoint for a fake transaction update.
func (s *CatalogStore) GetMCPServerForUpdate(ctx context.Context, id uuid.UUID) (domain.MCPServer, error) {
	return s.GetMCPServer(ctx, id)
}

// ListMCPServers returns a keyset-paginated collection of fake MCP endpoints.
func (s *CatalogStore) ListMCPServers(ctx context.Context, options domain.PageOptions) ([]domain.MCPServer, error) {
	defer s.lock(ctx)()
	items := []domain.MCPServer{}
	for _, server := range s.servers {
		if before(server.CreatedAt, server.ID, options) {
			items = append(items, cloneMCPServer(server))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, nil
}

// UpdateMCPServer replaces one MCP endpoint and advances its revision.
func (s *CatalogStore) UpdateMCPServer(ctx context.Context, server domain.MCPServer) (domain.MCPServer, error) {
	defer s.lock(ctx)()
	old, ok := s.servers[server.ID]
	if !ok {
		return domain.MCPServer{}, domain.ErrNotFound
	}
	for _, existing := range s.servers {
		if existing.ID != server.ID && existing.Slug == server.Slug {
			return domain.MCPServer{}, domain.ErrConflict
		}
	}
	server.Revision = old.Revision + 1
	s.servers[server.ID] = cloneMCPServer(server)
	return cloneMCPServer(server), nil
}

// DeleteMCPServer removes one endpoint and its fake bindings.
func (s *CatalogStore) DeleteMCPServer(ctx context.Context, id uuid.UUID) error {
	defer s.lock(ctx)()
	if _, ok := s.servers[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.servers, id)
	for agentID, bindings := range s.bindings {
		filtered := bindings.MCPServerIDs[:0]
		for _, serverID := range bindings.MCPServerIDs {
			if serverID != id {
				filtered = append(filtered, serverID)
			}
		}
		bindings.MCPServerIDs = filtered
		s.bindings[agentID] = bindings
	}
	return nil
}

// RecordMCPServerSync applies a cached catalog when the revision still matches.
func (s *CatalogStore) RecordMCPServerSync(ctx context.Context, id uuid.UUID, revision int64, tools []domain.MCPTool, status domain.ConnectionStatus, failure *domain.ProviderFailure, at time.Time) (bool, error) {
	defer s.lock(ctx)()
	server, ok := s.servers[id]
	if !ok || server.Revision != revision {
		return false, nil
	}
	server.Tools = append([]domain.MCPTool{}, tools...)
	server.Status = status
	server.LastError = copyPointer(failure)
	server.LastSyncedAt = &at
	server.UpdatedAt = at
	s.servers[id] = cloneMCPServer(server)
	return true, nil
}

// GetAgentToolBindings returns the fake bindings for one agent.
func (s *CatalogStore) GetAgentToolBindings(ctx context.Context, agentID uuid.UUID) (domain.AgentToolBindings, error) {
	defer s.lock(ctx)()
	return cloneBindings(s.bindings[agentID]), nil
}

// ReplaceAgentToolBindings replaces the fake bindings for one agent.
func (s *CatalogStore) ReplaceAgentToolBindings(ctx context.Context, agentID uuid.UUID, bindings domain.AgentToolBindings) error {
	defer s.lock(ctx)()
	if _, ok := s.agents[agentID]; !ok {
		return domain.ErrNotFound
	}
	for _, id := range bindings.ToolIDs {
		if _, ok := s.tools[id]; !ok {
			return domain.ErrNotFound
		}
	}
	for _, id := range bindings.MCPServerIDs {
		if _, ok := s.servers[id]; !ok {
			return domain.ErrNotFound
		}
	}
	s.bindings[agentID] = cloneBindings(bindings)
	return nil
}

// ResolveAgentTools returns a coherent fake snapshot for one agent.
func (s *CatalogStore) ResolveAgentTools(ctx context.Context, agentID uuid.UUID) (outbound.AgentToolCatalog, error) {
	defer s.lock(ctx)()
	bindings := s.bindings[agentID]
	catalog := outbound.AgentToolCatalog{HTTPTools: make([]domain.HTTPTool, 0, len(bindings.ToolIDs)), MCPServers: make([]domain.MCPServer, 0, len(bindings.MCPServerIDs))}
	seenConnections := map[uuid.UUID]bool{}
	for _, id := range bindings.ToolIDs {
		tool := cloneHTTPTool(s.tools[id])
		catalog.HTTPTools = append(catalog.HTTPTools, tool)
		if tool.ConnectionID != nil && !seenConnections[*tool.ConnectionID] {
			connection, ok := s.connections[*tool.ConnectionID]
			if !ok {
				return outbound.AgentToolCatalog{}, domain.ErrNotFound
			}
			catalog.APIConnections = append(catalog.APIConnections, cloneAPIConnection(connection))
			seenConnections[*tool.ConnectionID] = true
		}
	}
	for _, id := range bindings.MCPServerIDs {
		catalog.MCPServers = append(catalog.MCPServers, cloneMCPServer(s.servers[id]))
	}
	sort.Slice(catalog.HTTPTools, func(i, j int) bool { return catalog.HTTPTools[i].Slug < catalog.HTTPTools[j].Slug })
	sort.Slice(catalog.APIConnections, func(i, j int) bool { return catalog.APIConnections[i].Slug < catalog.APIConnections[j].Slug })
	sort.Slice(catalog.MCPServers, func(i, j int) bool { return catalog.MCPServers[i].Slug < catalog.MCPServers[j].Slug })
	return catalog, nil
}

var (
	_ outbound.APIConnectionRepository    = (*CatalogStore)(nil)
	_ outbound.ToolRepository             = (*CatalogStore)(nil)
	_ outbound.MCPServerRepository        = (*CatalogStore)(nil)
	_ outbound.AgentToolBindingRepository = (*CatalogStore)(nil)
)
