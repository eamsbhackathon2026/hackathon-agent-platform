package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

var _ outbound.MCPServerRepository = (*Store)(nil)

// CreateMCPServer persists a validated MCP endpoint.
func (s *Store) CreateMCPServer(ctx context.Context, server domain.MCPServer) error {
	tools, err := encodeMCPTools(server.Tools)
	if err != nil {
		return err
	}
	return mapError(s.queries(ctx).CreateMCPServer(ctx, sqlcgen.CreateMCPServerParams{ID: dbID(server.ID), Slug: server.Slug, DisplayName: server.DisplayName, Url: server.URL, SecretHeadersCiphertext: server.SecretHeadersCiphertext, SecretHeaderNames: server.SecretHeaderNames, AllowedTools: allowedToolsValue(server.AllowedTools), ToolsCache: tools, Status: string(server.Status), LastError: failureJSON(server.LastError), LastSyncedAt: optionalTime(server.LastSyncedAt), CreatedAt: catalogTime(server.CreatedAt), UpdatedAt: catalogTime(server.UpdatedAt)}))
}

// GetMCPServer returns a saved MCP endpoint by ID.
func (s *Store) GetMCPServer(ctx context.Context, id uuid.UUID) (domain.MCPServer, error) {
	v, err := s.queries(ctx).GetMCPServer(ctx, dbID(id))
	if err != nil {
		return domain.MCPServer{}, mapError(err)
	}
	return mcpServerModel(v)
}

// GetMCPServerForUpdate returns and locks an endpoint in the active transaction.
func (s *Store) GetMCPServerForUpdate(ctx context.Context, id uuid.UUID) (domain.MCPServer, error) {
	v, err := s.queries(ctx).GetMCPServerForUpdate(ctx, dbID(id))
	if err != nil {
		return domain.MCPServer{}, mapError(err)
	}
	return mcpServerModel(v)
}

// ListMCPServers returns saved MCP endpoints using keyset pagination.
func (s *Store) ListMCPServers(ctx context.Context, options domain.PageOptions) ([]domain.MCPServer, error) {
	limit, before, id := page(options)
	rows, err := s.queries(ctx).ListMCPServers(ctx, sqlcgen.ListMCPServersParams{Limit: limit, BeforeTime: before, BeforeID: id})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.MCPServer, 0, len(rows))
	for _, row := range rows {
		server, modelErr := mcpServerModel(row)
		if modelErr != nil {
			return nil, modelErr
		}
		result = append(result, server)
	}
	return result, nil
}

// UpdateMCPServer persists a complete endpoint replacement and advances its revision.
func (s *Store) UpdateMCPServer(ctx context.Context, server domain.MCPServer) (domain.MCPServer, error) {
	tools, err := encodeMCPTools(server.Tools)
	if err != nil {
		return domain.MCPServer{}, err
	}
	v, err := s.queries(ctx).UpdateMCPServer(ctx, sqlcgen.UpdateMCPServerParams{ID: dbID(server.ID), Slug: server.Slug, DisplayName: server.DisplayName, Url: server.URL, SecretHeadersCiphertext: server.SecretHeadersCiphertext, SecretHeaderNames: server.SecretHeaderNames, AllowedTools: allowedToolsValue(server.AllowedTools), ToolsCache: tools, Status: string(server.Status), LastError: failureJSON(server.LastError), LastSyncedAt: optionalTime(server.LastSyncedAt), UpdatedAt: catalogTime(server.UpdatedAt)})
	if err != nil {
		return domain.MCPServer{}, mapError(err)
	}
	return mcpServerModel(v)
}

// DeleteMCPServer removes an MCP endpoint by ID.
func (s *Store) DeleteMCPServer(ctx context.Context, id uuid.UUID) error {
	return affected(s.queries(ctx).DeleteMCPServer(ctx, dbID(id)))
}

// RecordMCPServerSync stores a refreshed catalog when the configuration revision matches.
func (s *Store) RecordMCPServerSync(ctx context.Context, id uuid.UUID, revision int64, tools []domain.MCPTool, status domain.ConnectionStatus, failure *domain.ProviderFailure, at time.Time) (bool, error) {
	cache, err := encodeMCPTools(tools)
	if err != nil {
		return false, err
	}
	n, err := s.queries(ctx).RecordMCPServerSync(ctx, sqlcgen.RecordMCPServerSyncParams{ID: dbID(id), Revision: revision, ToolsCache: cache, Status: string(status), LastError: failureJSON(failure), LastSyncedAt: catalogTime(at)})
	return n > 0, mapError(err)
}

func allowedToolsValue(value *[]string) []string {
	if value == nil {
		return nil
	}
	return append([]string{}, (*value)...)
}
