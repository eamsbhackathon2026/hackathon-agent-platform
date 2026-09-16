package domain

import "time"

// MaxWebhookAttempts caps automatic callback attempts.
const MaxWebhookAttempts = 8

// NextWebhookAttempt applies capped exponential backoff with a supplied jitter
// in [-1, 1], which keeps the policy deterministic in tests.
func NextWebhookAttempt(attempt int, now time.Time, jitter float64) time.Time {
	if attempt < 1 {
		attempt = 1
	}
	if jitter < -1 {
		jitter = -1
	} else if jitter > 1 {
		jitter = 1
	}
	delay := 10 * time.Second
	for n := 1; n < attempt && delay < time.Hour; n++ {
		delay *= 2
		if delay > time.Hour {
			delay = time.Hour
		}
	}
	delay = time.Duration(float64(delay) * (1 + jitter*0.2))
	return now.Add(delay)
}
