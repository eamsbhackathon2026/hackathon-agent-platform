package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2idConfig bounds hashing cost and simultaneous memory usage.
type Argon2idConfig struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	Concurrency int
}

// DefaultArgon2idConfig uses the OWASP 19 MiB, two-pass, single-lane profile.
func DefaultArgon2idConfig() Argon2idConfig {
	return Argon2idConfig{MemoryKiB: 19 * 1024, Iterations: 2, Parallelism: 1, Concurrency: 8}
}

// Argon2idPasswordHasher creates and verifies bounded PHC password hashes.
type Argon2idPasswordHasher struct {
	config Argon2idConfig
	slots  chan struct{}
}

var errPasswordHash = errors.New("invalid password hash")

func validArgonParameters(memory, iterations uint32, parallelism uint8) bool {
	return memory >= 19*1024 && memory <= 64*1024 && iterations >= 2 && iterations <= 10 && parallelism >= 1 && parallelism <= 4
}

// NewArgon2idPasswordHasher rejects weak or excessive cost parameters.
func NewArgon2idPasswordHasher(config Argon2idConfig) (*Argon2idPasswordHasher, error) {
	if !validArgonParameters(config.MemoryKiB, config.Iterations, config.Parallelism) || config.Concurrency < 1 || config.Concurrency > 8 {
		return nil, errors.New("unsafe argon2id configuration")
	}
	return &Argon2idPasswordHasher{config: config, slots: make(chan struct{}, config.Concurrency)}, nil
}

func (h *Argon2idPasswordHasher) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case h.slots <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-h.slots
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Hash uses a fresh salt and observes cancellation before and after Argon2.
func (h *Argon2idPasswordHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer func() { <-h.slots }()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("password salt generation failed")
	}
	c := h.config
	key := argon2.IDKey([]byte(password), salt, c.Iterations, c.MemoryKiB, c.Parallelism, 32)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", c.MemoryKiB, c.Iterations, c.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify compares the password to a PHC hash without secret-dependent equality.
func (h *Argon2idPasswordHasher) Verify(ctx context.Context, password, encodedHash string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	c, salt, expected, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer func() { <-h.slots }()
	actual := argon2.IDKey([]byte(password), salt, c.Iterations, c.MemoryKiB, c.Parallelism, 32)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func parsePasswordHash(encoded string) (Argon2idConfig, []byte, []byte, error) {
	var config Argon2idConfig
	if len(encoded) > 256 {
		return config, nil, nil, errPasswordHash
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return config, nil, nil, errPasswordHash
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return config, nil, nil, errPasswordHash
	}
	values := [3]uint32{}
	for i, prefix := range []string{"m=", "t=", "p="} {
		value, ok := strings.CutPrefix(params[i], prefix)
		if !ok {
			return config, nil, nil, errPasswordHash
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil || parsed > 65536 {
			return config, nil, nil, errPasswordHash
		}
		values[i] = uint32(parsed)
	}
	if values[0] > 64*1024 || values[1] > 10 || values[2] > 4 {
		return config, nil, nil, errPasswordHash
	}
	parallelism := values[2]
	if parallelism > 4 {
		return config, nil, nil, errPasswordHash
	}
	if !validArgonParameters(values[0], values[1], uint8(parallelism)) {
		return config, nil, nil, errPasswordHash
	}
	config.MemoryKiB, config.Iterations, config.Parallelism = values[0], values[1], uint8(parallelism)
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) < 16 || len(salt) > 32 {
		return config, nil, nil, errPasswordHash
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(key) != 32 {
		return config, nil, nil, errPasswordHash
	}
	return config, salt, key, nil
}
