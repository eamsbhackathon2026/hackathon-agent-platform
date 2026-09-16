package security

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

func TestArgon2id(t *testing.T) {
	h, err := NewArgon2idPasswordHasher(DefaultArgon2idConfig())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := h.Hash(context.Background(), "password with unicode: đ")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		password string
		want     bool
	}{{"password with unicode: đ", true}, {"wrong password", false}} {
		got, err := h.Verify(context.Background(), tc.password, encoded)
		if err != nil || got != tc.want {
			t.Fatalf("verify=%v err=%v", got, err)
		}
	}
	for _, malformed := range []string{"", strings.Repeat("x", 257), strings.Replace(encoded, "m=19456", "m=4294967295", 1), strings.Replace(encoded, "p=1", "p=257", 1), strings.Replace(encoded, "v=19", "v=18", 1), encoded + "$extra", strings.Replace(encoded, "t=2", "t=0", 1)} {
		if ok, err := h.Verify(context.Background(), "password", malformed); ok || err == nil {
			t.Fatal("accepted malformed hash")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Hash(ctx, "password"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := h.Verify(ctx, "password", encoded); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for range cap(h.slots) {
		h.slots <- struct{}{}
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := h.Hash(ctx, "password"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	for range cap(h.slots) {
		<-h.slots
	}
	if _, err := NewArgon2idPasswordHasher(Argon2idConfig{}); err == nil {
		t.Fatal("accepted unsafe parameters")
	}
}

func TestJWT(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	c := &testClock{now: time.Unix(1800000000, 0)}
	j, err := NewJWTAccessTokenIssuer(key, "issuer", "audience", c)
	if err != nil {
		t.Fatal(err)
	}
	user, session := uuid.New(), uuid.New()
	encoded, expires, err := j.Issue(user, session)
	if err != nil || !expires.Equal(c.now.Add(15*time.Minute)) {
		t.Fatalf("issue: %v", err)
	}
	claims, err := j.Verify(encoded)
	if err != nil || claims.UserID != user || claims.SessionID != session {
		t.Fatalf("verify: %+v %v", claims, err)
	}
	wireClaims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(encoded, wireClaims, func(*jwt.Token) (any, error) { return key, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithTimeFunc(c.Now))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tid", "role", "status"} {
		if _, exists := wireClaims[field]; exists {
			t.Fatalf("unexpected token claim: %s", field)
		}
	}
	base := func() jwt.MapClaims {
		return jwt.MapClaims{"iss": "issuer", "aud": "audience", "exp": c.now.Add(time.Hour).Unix(), "sub": user.String(), "sid": session.String()}
	}
	for _, field := range []string{"iss", "aud", "exp", "sub", "sid"} {
		t.Run("missing_"+field, func(t *testing.T) {
			claims := base()
			delete(claims, field)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := j.Verify(token); err == nil {
				t.Fatal("accepted missing claim")
			}
		})
	}
	for _, field := range []string{"iss", "aud", "sub", "sid"} {
		claims := base()
		claims[field] = "wrong"
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := j.Verify(token); err == nil {
			t.Fatal("accepted wrong claim")
		}
	}
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512, jwt.SigningMethodNone} {
		var signingKey any = key
		if method == jwt.SigningMethodNone {
			signingKey = jwt.UnsafeAllowNoneSignatureType
		}
		token, err := jwt.NewWithClaims(method, base()).SignedString(signingKey)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := j.Verify(token); err == nil {
			t.Fatal("accepted wrong algorithm")
		}
	}
	if _, err := j.Verify(encoded + "tampered"); err == nil {
		t.Fatal("accepted tampered token")
	}
	c.now = expires
	if _, err := j.Verify(encoded); err == nil {
		t.Fatal("accepted expired token")
	}
	if _, err := NewJWTAccessTokenIssuer(key[:31], "issuer", "audience", c); err == nil {
		t.Fatal("accepted short key")
	}
}

func TestSecretCipherAndHMAC(t *testing.T) {
	key := bytes.Repeat([]byte{4}, 32)
	cipher, err := NewAESGCMSecretCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	plain, aad := []byte("webhook-secret"), []byte("webhook:record")
	encrypted, err := cipher.Encrypt(plain, aad)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := cipher.Decrypt(encrypted, aad)
	if err != nil || !bytes.Equal(decoded, plain) {
		t.Fatalf("decrypt: %v", err)
	}
	other, err := cipher.Encrypt(plain, aad)
	if err != nil || bytes.Equal(encrypted, other) {
		t.Fatal("nonce reused")
	}
	for i := range encrypted {
		tampered := bytes.Clone(encrypted)
		tampered[i] ^= 1
		if _, err := cipher.Decrypt(tampered, aad); err == nil {
			t.Fatal("accepted tampering")
		}
	}
	for _, badAAD := range [][]byte{nil, []byte("webhook:other-record"), []byte("provider:record")} {
		if _, err := cipher.Decrypt(encrypted, badAAD); err == nil {
			t.Fatal("accepted wrong AAD")
		}
	}
	if _, err := cipher.Encrypt(plain, nil); err == nil {
		t.Fatal("accepted missing AAD")
	}
	if _, err := cipher.Decrypt(nil, aad); err == nil {
		t.Fatal("accepted empty ciphertext")
	}
	for _, size := range []int{16, 24, 31, 33} {
		if _, err := NewAESGCMSecretCipher(make([]byte, size)); err == nil {
			t.Fatal("accepted invalid key size")
		}
	}
	h, err := NewHMACAPIKeyHasher(key)
	if err != nil {
		t.Fatal(err)
	}
	hash := h.Hash("api-key")
	key[0] ^= 1
	if !h.Verify("api-key", hash) || h.Verify("wrong", hash) || h.Verify("api-key", hash[:31]) {
		t.Fatal("HMAC verification mismatch")
	}
	if _, err := NewHMACAPIKeyHasher(key[:31]); err == nil {
		t.Fatal("accepted short pepper")
	}
}

func TestRandomTokens(t *testing.T) {
	r := NewRandomToken()
	for _, length := range []int{1, 8, 43, 4096} {
		s, err := r.Base62(length)
		if err != nil || len(s) != length || strings.ContainsFunc(s, func(c rune) bool {
			return !strings.ContainsRune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", c)
		}) {
			t.Fatalf("invalid base62 output: length=%d err=%v", len(s), err)
		}
	}
	for _, length := range []int{-1, 0, 4097} {
		if _, err := r.Bytes(length); err == nil {
			t.Fatal("accepted invalid length")
		}
		if _, err := r.Base62(length); err == nil {
			t.Fatal("accepted invalid length")
		}
	}
	a, err := r.Bytes(32)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Bytes(32)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("repeated random token")
	}
}
