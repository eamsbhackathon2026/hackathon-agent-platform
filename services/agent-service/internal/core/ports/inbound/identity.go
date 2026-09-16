package inbound

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// Me contains the current user profile.
type Me struct {
	User domain.User
}

// AuthSession contains newly issued tokens and their expiration times.
type AuthSession struct {
	Me               Me
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// RegisterCommand contains the first owner account inputs.
type RegisterCommand struct{ Name, Email, Password string }

// LoginCommand contains credentials and the client address for rate limiting.
type LoginCommand struct{ Email, Password, IP string }

// ChangePasswordCommand contains the current password and its replacement.
type ChangePasswordCommand struct{ CurrentPassword, NewPassword string }

// AuthUseCase defines account and session workflows.
type AuthUseCase interface {
	SignupAllowed(context.Context) (bool, error)
	Register(context.Context, RegisterCommand) (AuthSession, error)
	Login(context.Context, LoginCommand) (AuthSession, error)
	Refresh(context.Context, string) (AuthSession, error)
	Logout(context.Context, string) error
	GetMe(context.Context, domain.Principal) (Me, error)
	ChangePassword(context.Context, domain.Principal, ChangePasswordCommand) error
}

// PageRequest contains client pagination parameters.
type PageRequest struct {
	Limit  int
	Cursor string
}

// MemberPage contains one member page and its continuation cursor.
type MemberPage struct {
	Items      []domain.User
	NextCursor *string
}

// MemberCreateCommand contains manager-supplied account details.
type MemberCreateCommand struct {
	Email, Name string
	Role        domain.Role
}

// MemberCreated returns a member and its temporary password once.
type MemberCreated struct {
	Member            domain.User
	TemporaryPassword string
}

// MemberUseCase defines workspace membership management.
type MemberUseCase interface {
	ListMembers(context.Context, domain.Principal, PageRequest) (MemberPage, error)
	CreateMember(context.Context, domain.Principal, MemberCreateCommand) (MemberCreated, error)
	UpdateMember(context.Context, domain.Principal, uuid.UUID, domain.MemberChanges) (domain.User, error)
}

// APIKeyPage contains safe access-key metadata and a continuation cursor.
type APIKeyPage struct {
	Items      []domain.APIKey
	NextCursor *string
}

// APIKeyCreateCommand contains the name and requested run scopes.
type APIKeyCreateCommand struct {
	Name   string
	Scopes []string
}

// APIKeyCreated returns newly generated secrets exactly once.
type APIKeyCreated struct {
	APIKey             domain.APIKey
	Key, WebhookSecret string
}

// APIKeyUseCase defines access-key lifecycle workflows.
type APIKeyUseCase interface {
	ListAPIKeys(context.Context, domain.Principal, PageRequest) (APIKeyPage, error)
	CreateAPIKey(context.Context, domain.Principal, APIKeyCreateCommand) (APIKeyCreated, error)
	RevokeAPIKey(context.Context, domain.Principal, uuid.UUID) error
}

// PrincipalResolver loads current authorization state from authentication credentials.
type PrincipalResolver interface {
	FromAccessToken(context.Context, string) (domain.Principal, error)
	FromAPIKey(context.Context, string) (domain.Principal, error)
}
