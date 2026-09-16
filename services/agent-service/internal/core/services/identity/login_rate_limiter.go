package identity

import (
	"crypto/sha256"
	"sync"
	"time"
)

type loginWindow struct {
	start time.Time
	count int
}
type loginRateLimiter struct {
	mu      sync.Mutex
	entries map[[32]byte]loginWindow
}

func (l *loginRateLimiter) allow(email, ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[[32]byte]loginWindow)
	}
	for k, v := range l.entries {
		if now.Sub(v.start) >= 15*time.Minute {
			delete(l.entries, k)
		}
	}
	key := sha256.Sum256([]byte(email + "\x00" + ip))
	v, ok := l.entries[key]
	if !ok {
		if len(l.entries) >= 10000 {
			return false
		}
		v.start = now
	}
	if v.count >= 10 {
		return false
	}
	v.count++
	l.entries[key] = v
	return true
}
