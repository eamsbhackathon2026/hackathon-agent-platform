package delivery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// ServiceDependencies supplies delivery history and validation boundaries.
type ServiceDependencies struct {
	Deliveries outbound.WebhookDeliveryRepository
	Runs       outbound.RunRepository
	APIKeys    outbound.APIKeyRepository
	URLs       outbound.URLValidator
	Clock      outbound.Clock
}

// Service exposes admin-only callback history and retry.
type Service struct{ ServiceDependencies }

// NewService validates delivery use-case dependencies.
func NewService(d ServiceDependencies) (*Service, error) {
	if d.Deliveries == nil || d.Runs == nil || d.APIKeys == nil || d.URLs == nil || d.Clock == nil {
		return nil, errors.New("webhook delivery dependencies are required")
	}
	return &Service{ServiceDependencies: d}, nil
}

// ListWebhookDeliveries returns one descending page after checking the run exists.
func (s *Service) ListWebhookDeliveries(ctx context.Context, p domain.Principal, runID uuid.UUID, request inbound.PageRequest) (page inbound.WebhookDeliveryPage, err error) {
	if err = domain.Authorize(p, domain.ActionWebhookDeliveries, nil); err != nil {
		return
	}
	if _, err = s.Runs.GetRun(ctx, runID); err != nil {
		return
	}
	limit := request.Limit
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return page, domain.Invalid("limit", "Giới hạn phải từ 1 đến 100.")
	}
	options := domain.PageOptions{Limit: limit + 1}
	if request.Cursor != "" {
		var cursor domain.PageCursor
		raw, decodeErr := base64.RawURLEncoding.DecodeString(request.Cursor)
		if decodeErr != nil || json.Unmarshal(raw, &cursor) != nil || cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
			return page, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
		}
		options.Before = &cursor
	}
	page.Items, err = s.Deliveries.ListWebhookDeliveries(ctx, runID, options)
	if err != nil {
		return
	}
	if len(page.Items) == options.Limit {
		page.Items = page.Items[:limit]
		last := page.Items[len(page.Items)-1]
		raw, _ := json.Marshal(domain.PageCursor{ID: last.ID, CreatedAt: last.CreatedAt})
		cursor := base64.RawURLEncoding.EncodeToString(raw)
		page.NextCursor = &cursor
	}
	return
}

// RetryWebhookDelivery resets a failed callback after rechecking key and URL.
func (s *Service) RetryWebhookDelivery(ctx context.Context, p domain.Principal, id uuid.UUID) (domain.WebhookDelivery, error) {
	if err := domain.Authorize(p, domain.ActionWebhookDeliveries, nil); err != nil {
		return domain.WebhookDelivery{}, err
	}
	delivery, err := s.Deliveries.GetWebhookDelivery(ctx, id)
	if err != nil {
		return domain.WebhookDelivery{}, err
	}
	if delivery.Status != domain.WebhookFailed {
		return domain.WebhookDelivery{}, domain.ErrConflict
	}
	key, err := s.APIKeys.GetAPIKey(ctx, delivery.APIKeyID)
	if err != nil {
		return domain.WebhookDelivery{}, err
	}
	if key.RevokedAt != nil {
		return domain.WebhookDelivery{}, domain.ErrConflict
	}
	if err = s.URLs.ValidateURL(delivery.URL); err != nil {
		return domain.WebhookDelivery{}, err
	}
	return s.Deliveries.ResetWebhookDelivery(ctx, id, s.Clock.Now())
}

var _ inbound.WebhookDeliveryUseCase = (*Service)(nil)
