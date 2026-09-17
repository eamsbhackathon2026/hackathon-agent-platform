package postgres

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

var _ outbound.APIConnectionRepository = (*Store)(nil)

// CreateAPIConnection persists one validated reusable API connection.
func (s *Store) CreateAPIConnection(ctx context.Context, connection domain.APIConnection) error {
	return mapError(s.queries(ctx).CreateAPIConnection(ctx, sqlcgen.CreateAPIConnectionParams{
		ID: dbID(connection.ID), Slug: connection.Slug, DisplayName: connection.DisplayName,
		BaseUrl: connection.BaseURL, PublicHeaders: encodeStringMap(connection.PublicHeaders),
		SecretHeadersCiphertext: connection.SecretHeadersCiphertext, SecretHeaderNames: connection.SecretHeaderNames,
		CreatedAt: catalogTime(connection.CreatedAt), UpdatedAt: catalogTime(connection.UpdatedAt),
	}))
}

// GetAPIConnection returns a reusable API connection by ID.
func (s *Store) GetAPIConnection(ctx context.Context, id uuid.UUID) (domain.APIConnection, error) {
	value, err := s.queries(ctx).GetAPIConnection(ctx, dbID(id))
	if err != nil {
		return domain.APIConnection{}, mapError(err)
	}
	return apiConnectionModel(value)
}

// GetAPIConnectionForUpdate returns and locks a connection in the active transaction.
func (s *Store) GetAPIConnectionForUpdate(ctx context.Context, id uuid.UUID) (domain.APIConnection, error) {
	value, err := s.queries(ctx).GetAPIConnectionForUpdate(ctx, dbID(id))
	if err != nil {
		return domain.APIConnection{}, mapError(err)
	}
	return apiConnectionModel(value)
}

// ListAPIConnections returns reusable API connections using keyset pagination.
func (s *Store) ListAPIConnections(ctx context.Context, options domain.PageOptions) ([]domain.APIConnection, error) {
	limit, before, id := page(options)
	rows, err := s.queries(ctx).ListAPIConnections(ctx, sqlcgen.ListAPIConnectionsParams{Limit: limit, BeforeTime: before, BeforeID: id})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.APIConnection, 0, len(rows))
	for _, row := range rows {
		connection, modelErr := apiConnectionModel(row)
		if modelErr != nil {
			return nil, modelErr
		}
		result = append(result, connection)
	}
	return result, nil
}

// ListAllAPIConnections returns every saved API connection ordered by slug.
func (s *Store) ListAllAPIConnections(ctx context.Context) ([]domain.APIConnection, error) {
	rows, err := s.queries(ctx).ListAllAPIConnections(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.APIConnection, 0, len(rows))
	for _, row := range rows {
		connection, modelErr := apiConnectionModel(row)
		if modelErr != nil {
			return nil, modelErr
		}
		result = append(result, connection)
	}
	return result, nil
}

// UpdateAPIConnection persists a complete connection replacement.
func (s *Store) UpdateAPIConnection(ctx context.Context, connection domain.APIConnection) error {
	n, err := s.queries(ctx).UpdateAPIConnection(ctx, sqlcgen.UpdateAPIConnectionParams{
		ID: dbID(connection.ID), Slug: connection.Slug, DisplayName: connection.DisplayName,
		BaseUrl: connection.BaseURL, PublicHeaders: encodeStringMap(connection.PublicHeaders),
		SecretHeadersCiphertext: connection.SecretHeadersCiphertext, SecretHeaderNames: connection.SecretHeaderNames,
		UpdatedAt: catalogTime(connection.UpdatedAt),
	})
	return affected(n, err)
}

// DeleteAPIConnection removes an unused API connection.
func (s *Store) DeleteAPIConnection(ctx context.Context, id uuid.UUID) error {
	return affected(s.queries(ctx).DeleteAPIConnection(ctx, dbID(id)))
}

// CountToolsByAPIConnection returns how many HTTP tools reference the connection.
func (s *Store) CountToolsByAPIConnection(ctx context.Context, id uuid.UUID) (int64, error) {
	count, err := s.queries(ctx).CountToolsByAPIConnection(ctx, dbID(id))
	return count, mapError(err)
}

func apiConnectionModel(value sqlcgen.ApiConnection) (domain.APIConnection, error) {
	headers, err := decodeStringMap(value.PublicHeaders)
	if err != nil {
		return domain.APIConnection{}, err
	}
	return domain.APIConnection{
		ID: uuid.UUID(value.ID.Bytes), Slug: value.Slug, DisplayName: value.DisplayName,
		BaseURL: value.BaseUrl, PublicHeaders: headers,
		SecretHeadersCiphertext: append([]byte(nil), value.SecretHeadersCiphertext...),
		SecretHeaderNames:       append([]string{}, value.SecretHeaderNames...),
		CreatedAt:               value.CreatedAt.Time.UTC(), UpdatedAt: value.UpdatedAt.Time.UTC(),
	}, nil
}
