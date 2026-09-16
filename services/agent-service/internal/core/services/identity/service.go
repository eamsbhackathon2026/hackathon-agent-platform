package identity

import (
	"context"
	"errors"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// Dependencies supplies the storage and security ports used by identity workflows.
type Dependencies struct {
	Users         outbound.UserRepository
	RefreshTokens outbound.RefreshTokenRepository
	APIKeys       outbound.APIKeyRepository
	Tx            outbound.TxManager
	Passwords     outbound.PasswordHasher
	Tokens        outbound.AccessTokenIssuer
	KeyHasher     outbound.APIKeyHasher
	Cipher        outbound.SecretCipher
	Clock         outbound.Clock
	IDs           outbound.IDGenerator
	Random        outbound.RandomToken
}

// Service implements identity use cases for one shared workspace.
type Service struct {
	Dependencies
	dummyHash string
	limiter   loginRateLimiter
}

// NewService checks required dependencies and prepares the dummy password hash.
func NewService(d Dependencies) (*Service, error) {
	if d.Users == nil || d.RefreshTokens == nil || d.APIKeys == nil || d.Tx == nil || d.Passwords == nil || d.Tokens == nil || d.KeyHasher == nil || d.Cipher == nil || d.Clock == nil || d.IDs == nil || d.Random == nil {
		return nil, errors.New("identity dependencies are required")
	}
	hash, err := d.Passwords.Hash(context.Background(), "dummy-password-for-timing-only")
	if err != nil {
		return nil, err
	}
	return &Service{Dependencies: d, dummyHash: hash}, nil
}

// SignupAllowed reports whether the first owner still needs to be created.
func (s *Service) SignupAllowed(ctx context.Context) (bool, error) {
	n, err := s.Users.CountUsers(ctx)
	return n == 0, err
}

// GetMe loads the authenticated user profile.
func (s *Service) GetMe(ctx context.Context, p domain.Principal) (inbound.Me, error) {
	if err := domain.Authorize(p, domain.ActionMe, nil); err != nil {
		return inbound.Me{}, err
	}
	u, err := s.Users.GetUser(ctx, p.UserID)
	if err != nil {
		return inbound.Me{}, err
	}
	return inbound.Me{User: u}, nil
}

// FromAccessToken resolves role and status from storage for every request.
func (s *Service) FromAccessToken(ctx context.Context, token string) (domain.Principal, error) {
	c, err := s.Tokens.Verify(token)
	if err != nil {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	u, err := s.Users.GetUser(ctx, c.UserID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && u.Status != domain.UserActive) {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	if err != nil {
		return domain.Principal{}, err
	}
	return domain.Principal{Kind: domain.PrincipalUser, UserID: u.ID, SessionID: c.SessionID, Role: u.Role, MustChangePassword: u.MustChangePassword}, nil
}

const refreshLifetime = 30 * 24 * time.Hour

var _ inbound.AuthUseCase = (*Service)(nil)
var _ inbound.MemberUseCase = (*Service)(nil)
var _ inbound.APIKeyUseCase = (*Service)(nil)
var _ inbound.PrincipalResolver = (*Service)(nil)
