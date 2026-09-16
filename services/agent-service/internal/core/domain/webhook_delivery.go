package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WebhookDeliveryStatus is the durable delivery lifecycle.
type WebhookDeliveryStatus string

const (
	// WebhookPending is eligible for automatic delivery.
	WebhookPending WebhookDeliveryStatus = "pending"
	// WebhookDelivered records a successful 2xx response.
	WebhookDelivered WebhookDeliveryStatus = "delivered"
	// WebhookFailed requires an explicit retry.
	WebhookFailed WebhookDeliveryStatus = "failed"
)

// WebhookEvent identifies the terminal run notification.
type WebhookEvent string

const (
	// WebhookRunCompleted carries a successful run.
	WebhookRunCompleted WebhookEvent = "run.completed"
	// WebhookRunFailed carries a failed run.
	WebhookRunFailed WebhookEvent = "run.failed"
	// WebhookRunCancelled carries a cancelled run.
	WebhookRunCancelled WebhookEvent = "run.cancelled"
)

// WebhookDelivery is an outbox row and its public delivery history.
type WebhookDelivery struct {
	ID, RunID, APIKeyID uuid.UUID
	URL                 string
	Event               WebhookEvent
	Payload             json.RawMessage
	Status              WebhookDeliveryStatus
	Attempts            int
	NextAttemptAt       *time.Time
	LockedUntil         *time.Time
	LastStatusCode      *int
	LastError           *string
	DeliveredAt         *time.Time
	CreatedAt           time.Time
}

// TerminalWebhookEvent maps a final run status to its callback event.
func TerminalWebhookEvent(status RunStatus) WebhookEvent {
	switch status {
	case RunSucceeded:
		return WebhookRunCompleted
	case RunCancelled:
		return WebhookRunCancelled
	default:
		return WebhookRunFailed
	}
}
