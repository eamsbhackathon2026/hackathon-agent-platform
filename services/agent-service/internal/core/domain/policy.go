package domain

import "github.com/google/uuid"

// Action identifies a server-side authorization decision.
type Action string

// Actions are permission labels, never credentials.
const (
	ActionMe                Action = "me"
	ActionChangePassword    Action = "password:change"
	ActionLogout            Action = "logout"
	ActionMembersRead       Action = "members:read"
	ActionMembersWrite      Action = "members:write"
	ActionRolesWrite        Action = "roles:write"
	ActionAPIKeysWrite      Action = "api_keys:write"
	ActionAPIKeysRead       Action = "api_keys:read" // #nosec G101 -- permission label, not a credential.
	ActionWebhookDeliveries Action = "webhook_deliveries:read"
	ActionResourcesRead     Action = "resources:read"
	ActionResourcesWrite    Action = "resources:write"
	ActionRunsWrite         Action = "runs:write"
	ActionRunsRead          Action = "runs:read"
	ActionSessionsRead      Action = "sessions:read"
	ActionSpansRead         Action = "spans:read"
	ActionReportsRead       Action = "reports:read"
)

// Authorize is fail-closed; ownerID identifies the user or API key that owns a resource.
func Authorize(p Principal, a Action, ownerID *uuid.UUID) error {
	if p.Kind == PrincipalAPIKey {
		if p.APIKeyID == uuid.Nil {
			return ErrForbidden
		}
		for _, scope := range p.Scopes {
			if a == ActionRunsWrite && scope == "runs:write" {
				return nil
			}
			if a == ActionRunsRead && scope == "runs:read" && ownerID != nil && *ownerID == p.APIKeyID {
				return nil
			}
		}
		return ErrForbidden
	}
	if p.Kind != PrincipalUser || p.UserID == uuid.Nil || !ValidRole(p.Role) {
		return ErrForbidden
	}
	if a == ActionMe || a == ActionChangePassword || a == ActionLogout {
		return nil
	}
	if p.MustChangePassword {
		return ErrForbidden
	}
	switch a {
	case ActionResourcesRead, ActionRunsWrite:
		return nil
	case ActionMembersRead, ActionMembersWrite, ActionAPIKeysRead, ActionAPIKeysWrite, ActionWebhookDeliveries, ActionResourcesWrite, ActionReportsRead:
		if p.Role == RoleOwner || p.Role == RoleAdmin {
			return nil
		}
	case ActionRolesWrite:
		if p.Role == RoleOwner {
			return nil
		}
	case ActionRunsRead, ActionSessionsRead, ActionSpansRead:
		if p.Role == RoleOwner || p.Role == RoleAdmin || (ownerID != nil && *ownerID == p.UserID) {
			return nil
		}
	}
	return ErrForbidden
}
