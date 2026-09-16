package delivery

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"math/big"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

const (
	defaultDeliveryBatch = 20
	defaultDeliveryLease = 30 * time.Second
)

// DispatcherDependencies contains callback delivery boundaries.
type DispatcherDependencies struct {
	Deliveries outbound.WebhookDeliveryRepository
	APIKeys    outbound.APIKeyRepository
	Cipher     outbound.SecretCipher
	Signer     outbound.WebhookSigner
	Sender     outbound.WebhookSender
	Clock      outbound.Clock
	URLs       outbound.URLValidator
	Batch      int
	Lease      time.Duration
	Jitter     func() float64
}

// Dispatcher claims and sends due callback records.
type Dispatcher struct{ DispatcherDependencies }

// NewDispatcher validates all security-sensitive dependencies.
func NewDispatcher(d DispatcherDependencies) (*Dispatcher, error) {
	if d.Deliveries == nil || d.APIKeys == nil || d.Cipher == nil || d.Signer == nil || d.Sender == nil || d.Clock == nil || d.URLs == nil {
		return nil, errors.New("webhook dispatcher dependencies are required")
	}
	if d.Batch <= 0 {
		d.Batch = defaultDeliveryBatch
	}
	if d.Lease <= 0 {
		d.Lease = defaultDeliveryLease
	}
	if d.Jitter == nil {
		d.Jitter = secureJitter
	}
	return &Dispatcher{DispatcherDependencies: d}, nil
}

func secureJitter() float64 {
	value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(2_000_001))
	if err != nil {
		return 0
	}
	return float64(value.Int64())/1_000_000 - 1
}

// DispatchDue processes one claimed batch and returns its size.
func (d *Dispatcher) DispatchDue(ctx context.Context) (int, error) {
	items, err := d.Deliveries.ClaimDueWebhookDeliveries(ctx, d.Batch, d.Lease)
	if err != nil {
		return 0, err
	}
	for _, item := range items {
		if err = d.dispatch(ctx, item); err != nil {
			return len(items), err
		}
	}
	return len(items), nil
}

func (d *Dispatcher) dispatch(ctx context.Context, item domain.WebhookDelivery) error {
	key, err := d.APIKeys.GetAPIKey(ctx, item.APIKeyID)
	if err != nil || key.RevokedAt != nil || d.URLs.ValidateURL(item.URL) != nil {
		_, markErr := d.Deliveries.MarkWebhookFailed(ctx, item.ID, nil, "Khóa truy cập hoặc địa chỉ nhận kết quả không còn hợp lệ.")
		return markErr
	}
	secret, err := d.Cipher.Decrypt(key.WebhookSecretCiphertext, []byte("webhook:"+key.ID.String()))
	if err != nil {
		_, markErr := d.Deliveries.MarkWebhookFailed(ctx, item.ID, nil, "Không thể đọc khóa ký kết quả.")
		return markErr
	}
	now := d.Clock.Now()
	headers, err := d.Signer.Sign(item.ID, now, item.Payload, string(secret))
	if err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	status, sendErr := d.Sender.Send(sendCtx, item.URL, headers, item.Payload)
	cancel()
	if sendErr == nil && status >= 200 && status < 300 {
		_, err = d.Deliveries.MarkWebhookDelivered(ctx, item.ID, status, d.Clock.Now())
		return err
	}
	statusPtr := &status
	if status == 0 {
		statusPtr = nil
	}
	message := "Hệ thống nhận kết quả chưa phản hồi thành công."
	if status == 410 || item.Attempts >= domain.MaxWebhookAttempts {
		_, err = d.Deliveries.MarkWebhookFailed(ctx, item.ID, statusPtr, message)
		return err
	}
	next := domain.NextWebhookAttempt(item.Attempts, now, d.Jitter())
	_, err = d.Deliveries.MarkWebhookRetry(ctx, item.ID, statusPtr, message, next)
	return err
}

var _ inbound.WebhookDispatcher = (*Dispatcher)(nil)
