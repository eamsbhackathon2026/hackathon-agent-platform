package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role identifies a workspace membership role.
type Role string

// Supported workspace membership roles.
const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// UserStatus controls whether an account can authenticate.
type UserStatus string

// Supported account lifecycle states.
const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

// User stores account identity and current authorization state.
type User struct {
	ID                 uuid.UUID
	Email              string
	Name               string
	PasswordHash       string
	Role               Role
	Status             UserStatus
	MustChangePassword bool
	LastLoginAt        *time.Time
	CreatedAt          time.Time
}

// RefreshToken stores only the digest and lifecycle of an opaque session token.
type RefreshToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	FamilyID   uuid.UUID
	TokenHash  []byte
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID
	CreatedAt  time.Time
}

// APIKey stores access-key metadata and protected signing material.
type APIKey struct {
	ID                      uuid.UUID
	Name                    string
	Prefix                  string
	KeyHash                 []byte
	Scopes                  []string
	WebhookSecretCiphertext []byte
	CreatedBy               uuid.UUID
	LastUsedAt              *time.Time
	RevokedAt               *time.Time
	CreatedAt               time.Time
}

// MemberChanges contains optional membership updates.
type MemberChanges struct {
	Name   *string
	Role   *Role
	Status *UserStatus
}

// PageCursor identifies a stable position in descending creation order.
type PageCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// PageOptions controls repository keyset pagination.
type PageOptions struct {
	Limit  int
	Before *PageCursor
}
