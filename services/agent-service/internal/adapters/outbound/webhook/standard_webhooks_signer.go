package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// StandardWebhooksSigner implements the Standard Webhooks HMAC envelope.
type StandardWebhooksSigner struct{}

// Sign returns headers for the exact payload bytes supplied by the dispatcher.
func (StandardWebhooksSigner) Sign(id uuid.UUID, at time.Time, body []byte, secret string) (map[string]string, error) {
	encoded := strings.TrimPrefix(secret, "whsec_")
	if encoded == secret {
		return nil, errors.New("invalid webhook secret")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) == 0 {
		return nil, errors.New("invalid webhook secret")
	}
	timestamp := strconv.FormatInt(at.Unix(), 10)
	message := id.String() + "." + timestamp + "." + string(body)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return map[string]string{
		"webhook-id": id.String(), "webhook-timestamp": timestamp,
		"webhook-signature": "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}, nil
}

var _ outbound.WebhookSigner = StandardWebhooksSigner{}
