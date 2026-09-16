package fakes

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

// Clock is settable before a test operation. Do not mutate Time concurrently.
type Clock struct{ Time time.Time }

// Now returns the configured test time.
func (c Clock) Now() time.Time { return c.Time }

// IDs generates unique test identifiers.
type IDs struct{}

// NewID returns a random UUID.
func (IDs) NewID() (uuid.UUID, error) { return uuid.NewRandom() }

// Random generates opaque values for tests.
type Random struct{}

// Bytes returns random bytes.
func (Random) Bytes(n int) ([]byte, error) {
	v := make([]byte, n)
	_, err := rand.Read(v)
	return v, err
}

// Base62 returns a random alphanumeric test string.
func (r Random) Base62(n int) (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	v, err := r.Bytes(n)
	if err != nil {
		return "", err
	}
	for i := range v {
		v[i] = alphabet[int(v[i])%len(alphabet)]
	}
	return string(v), nil
}

// PasswordHasher is deliberately fast and unsuitable for production passwords.
type PasswordHasher struct{}

// Hash computes a fast test digest.
func (PasswordHasher) Hash(_ context.Context, value string) (string, error) {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:]), nil
}

// Verify checks a value using the test implementation.
func (p PasswordHasher) Verify(ctx context.Context, value, hash string) (bool, error) {
	actual, err := p.Hash(ctx, value)
	return actual == hash, err
}

// KeyHasher provides fast digests for test API keys.
type KeyHasher struct{}

// Hash computes a fast test digest.
func (KeyHasher) Hash(value string) []byte { v := sha256.Sum256([]byte(value)); return v[:] }

// Verify checks a value using the test implementation.
func (k KeyHasher) Verify(value string, hash []byte) bool { return bytes.Equal(k.Hash(value), hash) }

// Cipher preserves the AAD check without providing real confidentiality.
type Cipher struct{}

// Encrypt encodes plaintext with its associated data for tests.
func (Cipher) Encrypt(plain, aad []byte) ([]byte, error) { return json.Marshal([][]byte{aad, plain}) }

// Decrypt checks associated data and decodes test plaintext.
func (Cipher) Decrypt(ciphertext, aad []byte) ([]byte, error) {
	var v [][]byte
	if json.Unmarshal(ciphertext, &v) != nil || len(v) != 2 || !bytes.Equal(v[0], aad) {
		return nil, domain.ErrUnauthenticated
	}
	return v[1], nil
}

// Tokens uses unsigned JSON for tests only. Never use it to authenticate real requests.
type Tokens struct {
	Clock outbound.Clock
	TTL   time.Duration
}
type tokenPayload struct {
	Claims    domain.AccessClaims
	ExpiresAt time.Time
}

func (t Tokens) now() time.Time {
	if t.Clock != nil {
		return t.Clock.Now()
	}
	return time.Now()
}

// Issue creates an unsigned test token with an expiration.
func (t Tokens) Issue(user, session uuid.UUID) (string, time.Time, error) {
	ttl := t.TTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	expires := t.now().Add(ttl)
	v, err := json.Marshal(tokenPayload{domain.AccessClaims{UserID: user, SessionID: session}, expires})
	return base64.RawURLEncoding.EncodeToString(v), expires, err
}

// Verify checks a value using the test implementation.
func (t Tokens) Verify(token string) (domain.AccessClaims, error) {
	v, err := base64.RawURLEncoding.DecodeString(token)
	var p tokenPayload
	if err != nil || json.Unmarshal(v, &p) != nil || !t.now().Before(p.ExpiresAt) || p.Claims.UserID == uuid.Nil {
		return domain.AccessClaims{}, domain.ErrUnauthenticated
	}
	return p.Claims, nil
}

var (
	_ outbound.Clock             = Clock{}
	_ outbound.IDGenerator       = IDs{}
	_ outbound.RandomToken       = Random{}
	_ outbound.PasswordHasher    = PasswordHasher{}
	_ outbound.APIKeyHasher      = KeyHasher{}
	_ outbound.SecretCipher      = Cipher{}
	_ outbound.AccessTokenIssuer = Tokens{}
)
