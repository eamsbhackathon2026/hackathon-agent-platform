package identity

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
)

func tokenHash(raw string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != raw {
		return nil, domain.ErrUnauthenticated
	}
	h := sha256.Sum256([]byte(raw))
	return h[:], nil
}
func (s *Service) newSession(ctx context.Context, u domain.User, old domain.RefreshToken) (inbound.AuthSession, error) {
	var out inbound.AuthSession
	id, err := s.IDs.NewID()
	if err != nil {
		return out, err
	}
	family := old.FamilyID
	if family == uuid.Nil {
		family, err = s.IDs.NewID()
		if err != nil {
			return out, err
		}
	}
	raw, err := s.Random.Bytes(32)
	if err != nil {
		return out, err
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	hash, err := tokenHash(secret)
	if err != nil {
		return out, err
	}
	now := s.Clock.Now()
	expiry := old.ExpiresAt
	if expiry.IsZero() {
		expiry = now.Add(refreshLifetime)
	}
	token := domain.RefreshToken{ID: id, UserID: u.ID, FamilyID: family, TokenHash: hash, ExpiresAt: expiry, CreatedAt: now}
	access, accessExpiry, err := s.Tokens.Issue(u.ID, family)
	if err != nil {
		return out, err
	}
	if err = s.RefreshTokens.CreateRefreshToken(ctx, token); err != nil {
		return out, err
	}
	if old.ID != uuid.Nil {
		if err = s.RefreshTokens.RotateRefreshToken(ctx, old.ID, id, now); err != nil {
			return out, err
		}
	}
	return inbound.AuthSession{Me: inbound.Me{User: u}, AccessToken: access, AccessExpiresAt: accessExpiry, RefreshToken: secret, RefreshExpiresAt: expiry}, nil
}

// Refresh rotates a token and commits family revocation on token reuse.
func (s *Service) Refresh(ctx context.Context, raw string) (inbound.AuthSession, error) {
	var out inbound.AuthSession
	hash, err := tokenHash(raw)
	if err != nil {
		return out, err
	}
	old, err := s.RefreshTokens.FindRefreshToken(ctx, hash)
	if errors.Is(err, domain.ErrNotFound) {
		return out, domain.ErrUnauthenticated
	}
	if err != nil {
		return out, err
	}
	rejected := false
	err = s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		u, e := s.Users.GetUserForUpdate(ctx, old.UserID)
		if e != nil {
			return e
		}
		old, e = s.RefreshTokens.GetRefreshTokenForUpdate(ctx, old.ID)
		if e != nil {
			return e
		}
		now := s.Clock.Now()
		if old.RevokedAt != nil || old.ReplacedBy != nil {
			rejected = true
			return s.RefreshTokens.RevokeRefreshFamily(ctx, old.FamilyID, now)
		}
		if u.Status != domain.UserActive || !now.Before(old.ExpiresAt) {
			return domain.ErrUnauthenticated
		}
		out, e = s.newSession(ctx, u, old)
		return e
	})
	if err == nil && rejected {
		err = domain.ErrUnauthenticated
	}
	return out, err
}

// Logout idempotently revokes the supplied refresh token family.
func (s *Service) Logout(ctx context.Context, raw string) error {
	hash, err := tokenHash(raw)
	if err != nil {
		return nil
	}
	old, err := s.RefreshTokens.FindRefreshToken(ctx, hash)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, e := s.Users.GetUserForUpdate(ctx, old.UserID); e != nil {
			return e
		}
		return s.RefreshTokens.RevokeRefreshFamily(ctx, old.FamilyID, s.Clock.Now())
	})
}

// ChangePassword updates the password and revokes every other session family.
func (s *Service) ChangePassword(ctx context.Context, p domain.Principal, c inbound.ChangePasswordCommand) error {
	if err := domain.Authorize(p, domain.ActionChangePassword, nil); err != nil {
		return err
	}
	if err := domain.ValidatePassword(c.NewPassword); err != nil {
		return err
	}
	if domain.ValidatePassword(c.CurrentPassword) != nil {
		return domain.ErrUnauthenticated
	}
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		u, err := s.Users.GetUserForUpdate(ctx, p.UserID)
		if err != nil {
			return err
		}
		ok, err := s.Passwords.Verify(ctx, c.CurrentPassword, u.PasswordHash)
		if err != nil {
			return err
		}
		if !ok || u.Status != domain.UserActive {
			return domain.ErrUnauthenticated
		}
		hash, err := s.Passwords.Hash(ctx, c.NewPassword)
		if err != nil {
			return err
		}
		if err = s.Users.SetPassword(ctx, p.UserID, hash); err != nil {
			return err
		}
		return s.RefreshTokens.RevokeUserRefreshTokens(ctx, p.UserID, p.SessionID, s.Clock.Now())
	})
}
