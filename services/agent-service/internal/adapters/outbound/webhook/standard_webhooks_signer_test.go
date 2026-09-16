package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/webhook"
)

func TestStandardWebhooksSignerMatchesIndependentHMAC(t *testing.T) {
	id := uuid.MustParse("01994c90-9a12-7000-8000-000000000001")
	at := time.Unix(1_789_430_400, 0).UTC()
	body := []byte(`{"type":"run.completed"}`)
	key := []byte("01234567890123456789012345678901")
	headers, err := (webhook.StandardWebhooksSigner{}).Sign(id, at, body, "whsec_"+base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(id.String() + "." + headers["webhook-timestamp"] + "." + string(body)))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if headers["webhook-id"] != id.String() || !hmac.Equal([]byte(strings.TrimPrefix(headers["webhook-signature"], "v1,")), []byte(want)) {
		t.Fatalf("headers=%v", headers)
	}
}

func TestStandardWebhooksSignerRejectsMalformedSecret(t *testing.T) {
	if _, err := (webhook.StandardWebhooksSigner{}).Sign(uuid.New(), time.Now(), nil, "secret"); err == nil {
		t.Fatal("secret không có prefix được chấp nhận")
	}
}
