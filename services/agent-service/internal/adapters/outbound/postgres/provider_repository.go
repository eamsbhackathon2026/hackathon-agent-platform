package postgres

import (
	"context"
	"errors"
	"time"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var _ outbound.ProviderRepository = (*Store)(nil)

// LockCatalog serializes catalog mutations within the current transaction.
func (s *Store) LockCatalog(ctx context.Context) error {
	if _, ok := ctx.Value(txKey{s}).(pgx.Tx); !ok {
		return errors.New("catalog lock requires transaction")
	}
	return mapError(s.queries(ctx).LockCatalog(ctx))
}

// CreateProvider persists an encrypted connection with its initial revision.
func (s *Store) CreateProvider(ctx context.Context, v domain.Provider) error {
	return mapError(s.queries(ctx).CreateProvider(ctx, sqlcgen.CreateProviderParams{ID: dbID(v.ID), Name: v.Name, Kind: string(v.Kind), BaseUrl: textValue(v.BaseURL), ApiKeyCiphertext: v.APIKeyCiphertext, ApiKeyHint: textValue(v.APIKeyHint), DefaultModel: textValue(v.DefaultModel), Status: string(v.Status), LastError: failureJSON(v.LastError), LastCheckedAt: optionalTime(v.LastCheckedAt), CreatedAt: catalogTime(v.CreatedAt), UpdatedAt: catalogTime(v.UpdatedAt)}))
}

// GetProvider loads a connection by ID.
func (s *Store) GetProvider(ctx context.Context, id uuid.UUID) (domain.Provider, error) {
	v, err := s.queries(ctx).GetProvider(ctx, dbID(id))
	if err != nil {
		return domain.Provider{}, mapError(err)
	}
	return providerModel(v)
}

// GetProviderForUpdate locks a connection for the current transaction.
func (s *Store) GetProviderForUpdate(ctx context.Context, id uuid.UUID) (domain.Provider, error) {
	v, err := s.queries(ctx).GetProviderForUpdate(ctx, dbID(id))
	if err != nil {
		return domain.Provider{}, mapError(err)
	}
	return providerModel(v)
}

// ListProviders returns connections in descending creation order.
func (s *Store) ListProviders(ctx context.Context, p domain.PageOptions) ([]domain.Provider, error) {
	limit, before, id := page(p)
	rows, err := s.queries(ctx).ListProviders(ctx, sqlcgen.ListProvidersParams{Limit: limit, BeforeTime: before, BeforeID: id})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.Provider, 0, len(rows))
	for _, v := range rows {
		p, err := providerModel(v)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

// UpdateProvider replaces configuration and advances its revision.
func (s *Store) UpdateProvider(ctx context.Context, v domain.Provider) (domain.Provider, error) {
	row, err := s.queries(ctx).UpdateProvider(ctx, sqlcgen.UpdateProviderParams{ID: dbID(v.ID), Name: v.Name, Kind: string(v.Kind), BaseUrl: textValue(v.BaseURL), ApiKeyCiphertext: v.APIKeyCiphertext, ApiKeyHint: textValue(v.APIKeyHint), DefaultModel: textValue(v.DefaultModel), Status: string(v.Status), LastError: failureJSON(v.LastError), LastCheckedAt: optionalTime(v.LastCheckedAt), UpdatedAt: catalogTime(v.UpdatedAt)})
	if err != nil {
		return domain.Provider{}, mapError(err)
	}
	return providerModel(row)
}

// DeleteProvider detaches archived assistants; active references prevent deletion.
func (s *Store) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	return affected(s.queries(ctx).DeleteProvider(ctx, dbID(id)))
}

// RecordProviderCheck applies a result only to the configuration that was checked.
func (s *Store) RecordProviderCheck(ctx context.Context, id uuid.UUID, revision int64, status domain.ConnectionStatus, failure *domain.ProviderFailure, at time.Time) (bool, error) {
	n, err := s.queries(ctx).RecordProviderCheck(ctx, sqlcgen.RecordProviderCheckParams{ID: dbID(id), Revision: revision, Status: string(status), LastError: failureJSON(failure), LastCheckedAt: catalogTime(at)})
	return n > 0, mapError(err)
}
