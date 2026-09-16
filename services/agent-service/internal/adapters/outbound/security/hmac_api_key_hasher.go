package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
)

// HMACAPIKeyHasher protects API key digests with a server-held pepper.
type HMACAPIKeyHasher struct{ pepper []byte }

// NewHMACAPIKeyHasher validates and copies the pepper.
func NewHMACAPIKeyHasher(pepper []byte) (*HMACAPIKeyHasher, error) {
	if len(pepper) < 32 {
		return nil, errors.New("API key pepper must contain at least 32 bytes")
	}
	return &HMACAPIKeyHasher{pepper: append([]byte(nil), pepper...)}, nil
}

// Hash returns the HMAC-SHA256 digest of the entire API key.
func (h *HMACAPIKeyHasher) Hash(key string) []byte {
	mac := hmac.New(sha256.New, h.pepper)
	_, _ = mac.Write([]byte(key))
	return mac.Sum(nil)
}

// Verify compares the expected digest in constant time.
func (h *HMACAPIKeyHasher) Verify(key string, expected []byte) bool {
	return hmac.Equal(h.Hash(key), expected)
}
