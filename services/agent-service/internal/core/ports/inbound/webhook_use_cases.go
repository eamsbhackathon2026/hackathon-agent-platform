package inbound

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// WebhookDeliveryPage is one descending page of callback history.
type WebhookDeliveryPage struct {
	Items      []domain.WebhookDelivery
	NextCursor *string
}

// WebhookDeliveryUseCase exposes admin history and manual retry.
type WebhookDeliveryUseCase interface {
	ListWebhookDeliveries(context.Context, domain.Principal, uuid.UUID, PageRequest) (WebhookDeliveryPage, error)
	RetryWebhookDelivery(context.Context, domain.Principal, uuid.UUID) (domain.WebhookDelivery, error)
}

// WebhookDispatcher processes due outbox rows.
type WebhookDispatcher interface {
	DispatchDue(context.Context) (int, error)
}
