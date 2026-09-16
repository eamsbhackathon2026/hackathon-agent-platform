package identity

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"context"
	"errors"
	"strings"
)

// Register atomically creates the first and only self-registered owner.
func (s *Service) Register(ctx context.Context, c inbound.RegisterCommand) (inbound.AuthSession, error) {
	var out inbound.AuthSession
	email, err := domain.NormalizeEmail(c.Email)
	if err != nil {
		return out, err
	}
	if err = domain.ValidateName("name", c.Name); err != nil {
		return out, err
	}
	if err = domain.ValidatePassword(c.Password); err != nil {
		return out, err
	}
	hash, err := s.Passwords.Hash(ctx, c.Password)
	if err != nil {
		return out, err
	}
	uid, err := s.IDs.NewID()
	if err != nil {
		return out, err
	}
	now := s.Clock.Now()
	u := domain.User{ID: uid, Email: email, Name: strings.TrimSpace(c.Name), PasswordHash: hash, Role: domain.RoleOwner, Status: domain.UserActive, CreatedAt: now}
	err = s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if e := s.Users.LockIdentity(ctx); e != nil {
			return e
		}
		allowed, e := s.SignupAllowed(ctx)
		if e != nil {
			return e
		}
		if !allowed {
			return domain.ErrForbidden
		}
		if e = s.Users.CreateUser(ctx, u); e != nil {
			return e
		}
		out, e = s.newSession(ctx, u, domain.RefreshToken{})
		return e
	})
	return out, err
}

// Login verifies without a row lock, then checks the verified hash under lock.
func (s *Service) Login(ctx context.Context, c inbound.LoginCommand) (inbound.AuthSession, error) {
	var out inbound.AuthSession
	email, validationErr := domain.NormalizeEmail(c.Email)
	if validationErr != nil {
		email = strings.ToLower(strings.TrimSpace(c.Email))
	}
	if !s.limiter.allow(email, c.IP, s.Clock.Now()) {
		return out, domain.ErrRateLimited
	}
	if validationErr != nil || domain.ValidatePassword(c.Password) != nil {
		_, _ = s.Passwords.Verify(ctx, "invalid", s.dummyHash)
		return out, domain.ErrUnauthenticated
	}
	u, err := s.Users.FindUserByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		_, e := s.Passwords.Verify(ctx, c.Password, s.dummyHash)
		if e != nil {
			return out, e
		}
		return out, domain.ErrUnauthenticated
	}
	if err != nil {
		return out, err
	}
	verifiedHash := u.PasswordHash
	ok, err := s.Passwords.Verify(ctx, c.Password, verifiedHash)
	if err != nil {
		return out, err
	}
	if !ok {
		return out, domain.ErrUnauthenticated
	}
	err = s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var e error
		u, e = s.Users.GetUserForUpdate(ctx, u.ID)
		if e != nil {
			return e
		}
		if u.PasswordHash != verifiedHash || u.Status != domain.UserActive {
			return domain.ErrUnauthenticated
		}
		now := s.Clock.Now()
		if e = s.Users.TouchLastLogin(ctx, u.ID, now); e != nil {
			return e
		}
		u.LastLoginAt = &now
		out, e = s.newSession(ctx, u, domain.RefreshToken{})
		return e
	})
	return out, err
}
