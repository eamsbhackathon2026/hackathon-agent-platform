package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// PasswordHasher hashes passwords and verifies candidate values.
type PasswordHasher interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) (bool, error)
}

// AccessTokenIssuer issues and verifies short-lived user session tokens.
type AccessTokenIssuer interface {
	Issue(userID, sessionID uuid.UUID) (string, time.Time, error)
	Verify(string) (domain.AccessClaims, error)
}

// APIKeyHasher authenticates complete access keys with a keyed digest.
type APIKeyHasher interface {
	Hash(string) []byte
	Verify(string, []byte) bool
}

// SecretCipher encrypts secrets with mandatory associated context.
type SecretCipher interface {
	Encrypt(plain, aad []byte) ([]byte, error)
	Decrypt(ciphertext, aad []byte) ([]byte, error)
}

// Clock provides the current time to application workflows.
type Clock interface{ Now() time.Time }

// IDGenerator creates identifiers independently of storage.
type IDGenerator interface{ NewID() (uuid.UUID, error) }

// RandomToken provides cryptographically random bytes and base62 strings.
type RandomToken interface {
	Bytes(int) ([]byte, error)
	Base62(int) (string, error)
}
