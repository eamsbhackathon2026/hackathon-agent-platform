package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// WebhookDeliveryRepository persists and leases callback attempts.
type WebhookDeliveryRepository interface {
	InsertWebhookDelivery(context.Context, domain.WebhookDelivery) error
	ClaimDueWebhookDeliveries(context.Context, int, time.Duration) ([]domain.WebhookDelivery, error)
	MarkWebhookDelivered(context.Context, uuid.UUID, int, time.Time) (domain.WebhookDelivery, error)
	MarkWebhookRetry(context.Context, uuid.UUID, *int, string, time.Time) (domain.WebhookDelivery, error)
	MarkWebhookFailed(context.Context, uuid.UUID, *int, string) (domain.WebhookDelivery, error)
	ListWebhookDeliveries(context.Context, uuid.UUID, domain.PageOptions) ([]domain.WebhookDelivery, error)
	GetWebhookDelivery(context.Context, uuid.UUID) (domain.WebhookDelivery, error)
	ResetWebhookDelivery(context.Context, uuid.UUID, time.Time) (domain.WebhookDelivery, error)
}

// WebhookSender sends a signed payload through a guarded HTTP client.
type WebhookSender interface {
	Send(context.Context, string, map[string]string, []byte) (int, error)
}

// WebhookSigner creates Standard Webhooks headers for an exact payload.
type WebhookSigner interface {
	Sign(uuid.UUID, time.Time, []byte, string) (map[string]string, error)
}

// URLValidator applies deployment egress policy before data is persisted.
type URLValidator interface{ ValidateURL(string) error }
