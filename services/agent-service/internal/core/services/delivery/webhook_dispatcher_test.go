package delivery

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

type deliveryRepository struct{ item domain.WebhookDelivery }

func (r *deliveryRepository) InsertWebhookDelivery(context.Context, domain.WebhookDelivery) error {
	return nil
}
func (r *deliveryRepository) ClaimDueWebhookDeliveries(context.Context, int, time.Duration) ([]domain.WebhookDelivery, error) {
	r.item.Attempts++
	return []domain.WebhookDelivery{r.item}, nil
}
func (r *deliveryRepository) MarkWebhookDelivered(_ context.Context, _ uuid.UUID, status int, at time.Time) (domain.WebhookDelivery, error) {
	r.item.Status = domain.WebhookDelivered
	r.item.LastStatusCode = &status
	r.item.DeliveredAt = &at
	return r.item, nil
}
func (r *deliveryRepository) MarkWebhookRetry(_ context.Context, _ uuid.UUID, status *int, message string, at time.Time) (domain.WebhookDelivery, error) {
	r.item.LastStatusCode, r.item.LastError, r.item.NextAttemptAt = status, &message, &at
	return r.item, nil
}
func (r *deliveryRepository) MarkWebhookFailed(_ context.Context, _ uuid.UUID, status *int, message string) (domain.WebhookDelivery, error) {
	r.item.Status, r.item.LastStatusCode, r.item.LastError = domain.WebhookFailed, status, &message
	return r.item, nil
}
func (r *deliveryRepository) ListWebhookDeliveries(context.Context, uuid.UUID, domain.PageOptions) ([]domain.WebhookDelivery, error) {
	return nil, nil
}
func (r *deliveryRepository) GetWebhookDelivery(context.Context, uuid.UUID) (domain.WebhookDelivery, error) {
	return r.item, nil
}
func (r *deliveryRepository) ResetWebhookDelivery(context.Context, uuid.UUID, time.Time) (domain.WebhookDelivery, error) {
	return r.item, nil
}

type deliveryAPIKeys struct {
	outbound.APIKeyRepository
	key domain.APIKey
}

func (r deliveryAPIKeys) GetAPIKey(context.Context, uuid.UUID) (domain.APIKey, error) {
	return r.key, nil
}

type deliveryURLs struct{}

func (deliveryURLs) ValidateURL(string) error { return nil }

type deliverySigner struct{}

func (deliverySigner) Sign(uuid.UUID, time.Time, []byte, string) (map[string]string, error) {
	return map[string]string{"signature": "ok"}, nil
}

type deliverySender struct {
	status int
	err    error
}

func (s deliverySender) Send(context.Context, string, map[string]string, []byte) (int, error) {
	return s.status, s.err
}

func TestDispatcherTerminalAndRetryOutcomes(t *testing.T) {
	now := time.Date(2026, 9, 15, 1, 0, 0, 0, time.UTC)
	keyID := uuid.New()
	cipher := fakes.Cipher{}
	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	encrypted, _ := cipher.Encrypt([]byte(secret), []byte("webhook:"+keyID.String()))
	for _, test := range []struct {
		name       string
		status     int
		sendErr    error
		wantStatus domain.WebhookDeliveryStatus
		wantDelay  time.Duration
	}{
		{name: "success", status: 204, wantStatus: domain.WebhookDelivered},
		{name: "gone", status: 410, wantStatus: domain.WebhookFailed},
		{name: "server retry", status: 500, wantStatus: domain.WebhookPending, wantDelay: 10 * time.Second},
		{name: "network retry", sendErr: errors.New("offline"), wantStatus: domain.WebhookPending, wantDelay: 10 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &deliveryRepository{item: domain.WebhookDelivery{ID: uuid.New(), APIKeyID: keyID, URL: "https://example.test/hook", Payload: []byte(`{}`), Status: domain.WebhookPending, NextAttemptAt: &now}}
			dispatcher, err := NewDispatcher(DispatcherDependencies{Deliveries: repo, APIKeys: deliveryAPIKeys{key: domain.APIKey{ID: keyID, WebhookSecretCiphertext: encrypted}}, Cipher: cipher, Signer: deliverySigner{}, Sender: deliverySender{status: test.status, err: test.sendErr}, Clock: fakes.Clock{Time: now}, URLs: deliveryURLs{}, Jitter: func() float64 { return 0 }})
			if err != nil {
				t.Fatal(err)
			}
			if count, err := dispatcher.DispatchDue(t.Context()); err != nil || count != 1 {
				t.Fatalf("count=%d err=%v", count, err)
			}
			if repo.item.Status != test.wantStatus {
				t.Fatalf("status=%s want=%s", repo.item.Status, test.wantStatus)
			}
			if test.wantDelay > 0 && (repo.item.NextAttemptAt == nil || repo.item.NextAttemptAt.Sub(now) != test.wantDelay) {
				t.Fatalf("next=%v", repo.item.NextAttemptAt)
			}
		})
	}
}
