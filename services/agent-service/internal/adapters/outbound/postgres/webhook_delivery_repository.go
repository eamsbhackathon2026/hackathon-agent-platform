package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// InsertWebhookDelivery appends a callback outbox record in the current transaction.
func (s *Store) InsertWebhookDelivery(ctx context.Context, value domain.WebhookDelivery) error {
	if value.Attempts < 0 || value.Attempts > domain.MaxWebhookAttempts {
		return domain.ErrValidation
	}
	return mapError(s.queries(ctx).InsertWebhookDelivery(ctx, sqlcgen.InsertWebhookDeliveryParams{
		ID: dbID(value.ID), RunID: dbID(value.RunID), ApiKeyID: dbID(value.APIKeyID), Url: value.URL,
		Event: string(value.Event), Payload: value.Payload, Status: string(value.Status), Attempts: int32(value.Attempts),
		NextAttemptAt: optionalTime(value.NextAttemptAt), LockedUntil: optionalTime(value.LockedUntil),
		LastStatusCode: int4Value(value.LastStatusCode), LastError: textValue(value.LastError),
		DeliveredAt: optionalTime(value.DeliveredAt), CreatedAt: dbTime(value.CreatedAt),
	}))
}

// ClaimDueWebhookDeliveries leases due pending callbacks.
func (s *Store) ClaimDueWebhookDeliveries(ctx context.Context, limit int, lease time.Duration) ([]domain.WebhookDelivery, error) {
	if limit < 1 {
		limit = 1
	} else if limit > 100 {
		limit = 100
	}
	rows, err := s.queries(ctx).ClaimDueWebhookDeliveries(ctx, sqlcgen.ClaimDueWebhookDeliveriesParams{LeaseMilliseconds: lease.Milliseconds(), DeliveryLimit: int32(limit)})
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]domain.WebhookDelivery, 0, len(rows))
	for _, row := range rows {
		items = append(items, webhookDeliveryModel(row))
	}
	return items, nil
}

// MarkWebhookDelivered records a successful 2xx response.
func (s *Store) MarkWebhookDelivered(ctx context.Context, id uuid.UUID, status int, at time.Time) (domain.WebhookDelivery, error) {
	row, err := s.queries(ctx).MarkWebhookDelivered(ctx, sqlcgen.MarkWebhookDeliveredParams{ID: dbID(id), LastStatusCode: int4Value(&status), DeliveredAt: dbTime(at)})
	return webhookDeliveryModel(row), mapError(err)
}

// MarkWebhookRetry schedules another attempt.
func (s *Store) MarkWebhookRetry(ctx context.Context, id uuid.UUID, status *int, message string, at time.Time) (domain.WebhookDelivery, error) {
	row, err := s.queries(ctx).MarkWebhookRetry(ctx, sqlcgen.MarkWebhookRetryParams{ID: dbID(id), LastStatusCode: int4Value(status), LastError: textValue(&message), NextAttemptAt: dbTime(at)})
	return webhookDeliveryModel(row), mapError(err)
}

// MarkWebhookFailed stops automatic attempts.
func (s *Store) MarkWebhookFailed(ctx context.Context, id uuid.UUID, status *int, message string) (domain.WebhookDelivery, error) {
	row, err := s.queries(ctx).MarkWebhookFailed(ctx, sqlcgen.MarkWebhookFailedParams{ID: dbID(id), LastStatusCode: int4Value(status), LastError: textValue(&message)})
	return webhookDeliveryModel(row), mapError(err)
}

// ListWebhookDeliveries returns a descending page for one run.
func (s *Store) ListWebhookDeliveries(ctx context.Context, runID uuid.UUID, options domain.PageOptions) ([]domain.WebhookDelivery, error) {
	limit, before, beforeID := page(options)
	rows, err := s.queries(ctx).ListWebhookDeliveries(ctx, sqlcgen.ListWebhookDeliveriesParams{RunID: dbID(runID), BeforeTime: before, BeforeID: beforeID, DeliveryLimit: limit})
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]domain.WebhookDelivery, 0, len(rows))
	for _, row := range rows {
		items = append(items, webhookDeliveryModel(row))
	}
	return items, nil
}

// GetWebhookDelivery loads a callback by ID.
func (s *Store) GetWebhookDelivery(ctx context.Context, id uuid.UUID) (domain.WebhookDelivery, error) {
	row, err := s.queries(ctx).GetWebhookDelivery(ctx, dbID(id))
	return webhookDeliveryModel(row), mapError(err)
}

// ResetWebhookDelivery makes a failed callback immediately eligible.
func (s *Store) ResetWebhookDelivery(ctx context.Context, id uuid.UUID, at time.Time) (domain.WebhookDelivery, error) {
	row, err := s.queries(ctx).ResetWebhookDelivery(ctx, sqlcgen.ResetWebhookDeliveryParams{ID: dbID(id), NextAttemptAt: dbTime(at)})
	return webhookDeliveryModel(row), mapError(err)
}

var _ outbound.WebhookDeliveryRepository = (*Store)(nil)
