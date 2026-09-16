package security

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// AccessTokenLifetime limits an access token to fifteen minutes.
const AccessTokenLifetime = 15 * time.Minute

// JWTAccessTokenIssuer issues HS256 tokens with user and refresh-family IDs.
type JWTAccessTokenIssuer struct {
	key              []byte
	issuer, audience string
	clock            outbound.Clock
}

type accessClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
}

// NewJWTAccessTokenIssuer validates configuration and copies the signing key.
func NewJWTAccessTokenIssuer(key []byte, issuer, audience string, clock outbound.Clock) (*JWTAccessTokenIssuer, error) {
	if len(key) < 32 || strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" || clock == nil {
		return nil, errors.New("invalid access token configuration")
	}
	return &JWTAccessTokenIssuer{key: append([]byte(nil), key...), issuer: issuer, audience: audience, clock: clock}, nil
}

// Issue binds the token to the supplied refresh family without rotating it.
func (j *JWTAccessTokenIssuer) Issue(userID, sessionID uuid.UUID) (string, time.Time, error) {
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return "", time.Time{}, errors.New("invalid access token identifiers")
	}
	now := j.clock.Now().UTC().Truncate(time.Second)
	expires := now.Add(AccessTokenLifetime)
	claims := accessClaims{RegisteredClaims: jwt.RegisteredClaims{Issuer: j.issuer, Subject: userID.String(), Audience: jwt.ClaimStrings{j.audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)}, SessionID: sessionID.String()}
	encoded, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.key)
	if err != nil {
		return "", time.Time{}, errors.New("access token signing failed")
	}
	return encoded, expires, nil
}

// Verify checks the algorithm, signature, issuer, audience, expiry, and IDs.
func (j *JWTAccessTokenIssuer) Verify(encoded string) (domain.AccessClaims, error) {
	var result domain.AccessClaims
	if len(encoded) > 4096 {
		return result, domain.ErrUnauthenticated
	}
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(encoded, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrUnauthenticated
		}
		return j.key, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(j.issuer), jwt.WithAudience(j.audience), jwt.WithExpirationRequired(), jwt.WithTimeFunc(j.clock.Now), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return result, domain.ErrUnauthenticated
	}
	userID, userErr := uuid.Parse(claims.Subject)
	sessionID, sessionErr := uuid.Parse(claims.SessionID)
	if userErr != nil || sessionErr != nil || userID == uuid.Nil || sessionID == uuid.Nil {
		return result, domain.ErrUnauthenticated
	}
	return domain.AccessClaims{UserID: userID, SessionID: sessionID}, nil
}
