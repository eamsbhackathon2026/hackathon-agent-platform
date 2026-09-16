package security

import (
	"crypto/rand"
	"errors"
)

// RandomToken uses the operating system cryptographic random source.
type RandomToken struct{}

// NewRandomToken creates a cryptographic token generator.
func NewRandomToken() *RandomToken { return &RandomToken{} }

// Bytes returns between one and 4096 random bytes.
func (*RandomToken) Bytes(length int) ([]byte, error) {
	if length < 1 || length > 4096 {
		return nil, errors.New("invalid token length")
	}
	result := make([]byte, length)
	if _, err := rand.Read(result); err != nil {
		return nil, errors.New("random token generation failed")
	}
	return result, nil
}

// Base62 uses rejection sampling to avoid modulo bias.
func (r *RandomToken) Base62(length int) (string, error) {
	if length < 1 || length > 4096 {
		return "", errors.New("invalid token length")
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, length)
	for len(result) < length {
		batch, err := r.Bytes(length - len(result))
		if err != nil {
			return "", err
		}
		for _, b := range batch {
			if b < 248 {
				result = append(result, alphabet[int(b)%len(alphabet)])
			}
		}
	}
	return string(result), nil
}
