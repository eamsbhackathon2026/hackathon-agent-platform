package domain

import "github.com/google/uuid"

// PrincipalKind distinguishes user and API-key authentication.
type PrincipalKind string

// Supported authentication methods.
const (
	PrincipalUser   PrincipalKind = "user"
	PrincipalAPIKey PrincipalKind = "api_key"
)

// Principal carries explicitly resolved identity into application services.
type Principal struct {
	Kind               PrincipalKind
	UserID             uuid.UUID
	APIKeyID           uuid.UUID
	SessionID          uuid.UUID
	Role               Role
	Scopes             []string
	MustChangePassword bool
}

// AccessClaims contains identifiers only; role and status are resolved from storage.
type AccessClaims struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}
