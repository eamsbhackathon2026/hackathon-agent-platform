package domain_test

import (
	"bytes"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestIdempotencyRequestHashCanonicalizesJSONKeys(t *testing.T) {
	left, err := domain.IdempotencyRequestHash("POST", "/runs", []byte(`{"input":{"b":2,"a":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	right, err := domain.IdempotencyRequestHash("POST", "/runs", []byte(`{ "input": { "a": 1, "b": 2 } }`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(left, right) {
		t.Fatal("thứ tự khóa JSON làm thay đổi request hash")
	}
	other, _ := domain.IdempotencyRequestHash("POST", "/runs/other", []byte(`{"input":{"a":1,"b":2}}`))
	if bytes.Equal(left, other) {
		t.Fatal("path khác nhau dùng chung request hash")
	}
}

func TestNextWebhookAttemptBackoffAndJitter(t *testing.T) {
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	wants := []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second, 160 * time.Second, 320 * time.Second, 640 * time.Second, 1280 * time.Second, 2560 * time.Second, time.Hour}
	for index, want := range wants {
		attempt := index + 1
		if got := domain.NextWebhookAttempt(attempt, now, 0).Sub(now); got != want {
			t.Fatalf("attempt %d: got %s want %s", attempt, got, want)
		}
		low := domain.NextWebhookAttempt(attempt, now, -1).Sub(now)
		high := domain.NextWebhookAttempt(attempt, now, 1).Sub(now)
		if low != time.Duration(float64(want)*0.8) || high != time.Duration(float64(want)*1.2) {
			t.Fatalf("attempt %d jitter [%s,%s]", attempt, low, high)
		}
	}
}
