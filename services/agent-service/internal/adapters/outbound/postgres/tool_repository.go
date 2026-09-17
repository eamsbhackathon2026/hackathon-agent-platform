package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

var _ outbound.ToolRepository = (*Store)(nil)

// LockTooling serializes configuration and binding mutations in one transaction.
func (s *Store) LockTooling(ctx context.Context) error {
	if _, ok := ctx.Value(txKey{s}).(pgx.Tx); !ok {
		return errors.New("tooling lock requires transaction")
	}
	return mapError(s.queries(ctx).LockTooling(ctx))
}

// CreateTool persists a validated HTTP tool.
func (s *Store) CreateTool(ctx context.Context, tool domain.HTTPTool) error {
	timeout, err := toolTimeoutSeconds(tool.TimeoutSeconds)
	if err != nil {
		return err
	}
	return mapError(s.queries(ctx).CreateTool(ctx, sqlcgen.CreateToolParams{ID: dbID(tool.ID), ConnectionID: optionalID(tool.ConnectionID), Slug: tool.Slug, DisplayName: tool.DisplayName, Description: tool.Description, Method: string(tool.Method), UrlTemplate: tool.URLTemplate, Params: encodeToolParams(tool.Params), PublicHeaders: encodeStringMap(tool.PublicHeaders), SecretHeadersCiphertext: tool.SecretHeadersCiphertext, SecretHeaderNames: tool.SecretHeaderNames, TimeoutSeconds: timeout, CreatedAt: catalogTime(tool.CreatedAt), UpdatedAt: catalogTime(tool.UpdatedAt)}))
}

// GetTool returns a saved HTTP tool by ID.
func (s *Store) GetTool(ctx context.Context, id uuid.UUID) (domain.HTTPTool, error) {
	v, err := s.queries(ctx).GetTool(ctx, dbID(id))
	if err != nil {
		return domain.HTTPTool{}, mapError(err)
	}
	return httpToolModel(v)
}

// GetToolForUpdate returns and locks a saved HTTP tool in the active transaction.
func (s *Store) GetToolForUpdate(ctx context.Context, id uuid.UUID) (domain.HTTPTool, error) {
	v, err := s.queries(ctx).GetToolForUpdate(ctx, dbID(id))
	if err != nil {
		return domain.HTTPTool{}, mapError(err)
	}
	return httpToolModel(v)
}

// ListTools returns saved HTTP tools using keyset pagination.
func (s *Store) ListTools(ctx context.Context, options domain.PageOptions) ([]domain.HTTPTool, error) {
	limit, before, id := page(options)
	rows, err := s.queries(ctx).ListTools(ctx, sqlcgen.ListToolsParams{Limit: limit, BeforeTime: before, BeforeID: id})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.HTTPTool, 0, len(rows))
	for _, row := range rows {
		tool, modelErr := httpToolModel(row)
		if modelErr != nil {
			return nil, modelErr
		}
		result = append(result, tool)
	}
	return result, nil
}

// ListAllTools returns every saved HTTP tool ordered by slug.
func (s *Store) ListAllTools(ctx context.Context) ([]domain.HTTPTool, error) {
	rows, err := s.queries(ctx).ListAllTools(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.HTTPTool, 0, len(rows))
	for _, row := range rows {
		tool, modelErr := httpToolModel(row)
		if modelErr != nil {
			return nil, modelErr
		}
		result = append(result, tool)
	}
	return result, nil
}

// UpdateTool persists a complete HTTP tool replacement.
func (s *Store) UpdateTool(ctx context.Context, tool domain.HTTPTool) error {
	timeout, err := toolTimeoutSeconds(tool.TimeoutSeconds)
	if err != nil {
		return err
	}
	n, err := s.queries(ctx).UpdateTool(ctx, sqlcgen.UpdateToolParams{ID: dbID(tool.ID), ConnectionID: optionalID(tool.ConnectionID), Slug: tool.Slug, DisplayName: tool.DisplayName, Description: tool.Description, Method: string(tool.Method), UrlTemplate: tool.URLTemplate, Params: encodeToolParams(tool.Params), PublicHeaders: encodeStringMap(tool.PublicHeaders), SecretHeadersCiphertext: tool.SecretHeadersCiphertext, SecretHeaderNames: tool.SecretHeaderNames, TimeoutSeconds: timeout, UpdatedAt: catalogTime(tool.UpdatedAt)})
	return affected(n, err)
}

// DeleteTool removes an HTTP tool by ID.
func (s *Store) DeleteTool(ctx context.Context, id uuid.UUID) error {
	return affected(s.queries(ctx).DeleteTool(ctx, dbID(id)))
}

func toolTimeoutSeconds(value int) (int32, error) {
	if value < 1 || value > 60 {
		return 0, domain.Invalid("timeout_seconds", "Thời gian chờ phải từ 1 đến 60 giây.")
	}
	return int32(value), nil
}
