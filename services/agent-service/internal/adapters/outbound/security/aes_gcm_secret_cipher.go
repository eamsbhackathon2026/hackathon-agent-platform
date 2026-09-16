// Package security implements cryptographic identity ports.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

// AESGCMSecretCipher authenticates secrets and their purpose-bound associated data.
type AESGCMSecretCipher struct{ aead cipher.AEAD }

// NewAESGCMSecretCipher requires an AES-256 key.
func NewAESGCMSecretCipher(key []byte) (*AESGCMSecretCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-256 key must contain exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("secret cipher initialization failed")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("secret cipher initialization failed")
	}
	return &AESGCMSecretCipher{aead: aead}, nil
}

// Encrypt produces a version byte, random nonce, and authenticated ciphertext.
func (s *AESGCMSecretCipher) Encrypt(plain, aad []byte) ([]byte, error) {
	if len(aad) == 0 {
		return nil, errors.New("secret encryption requires associated data")
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, errors.New("secret nonce generation failed")
	}
	result := append([]byte{1}, nonce...)
	return s.aead.Seal(result, nonce, plain, aad), nil
}

// Decrypt requires the same nonempty associated data used during encryption.
func (s *AESGCMSecretCipher) Decrypt(encrypted, aad []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(aad) == 0 || len(encrypted) < 1+n+s.aead.Overhead() || encrypted[0] != 1 {
		return nil, errors.New("invalid encrypted secret")
	}
	plain, err := s.aead.Open(nil, encrypted[1:1+n], encrypted[1+n:], aad)
	if err != nil {
		return nil, errors.New("invalid encrypted secret")
	}
	return plain, nil
}
