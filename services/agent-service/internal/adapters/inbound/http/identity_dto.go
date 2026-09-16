package http

import (
	"github.com/oapi-codegen/nullable"
	openapitypes "github.com/oapi-codegen/runtime/types"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

func userDTO(u domain.User) gen.User {
	return gen.User{Id: u.ID, Email: openapitypes.Email(u.Email), Name: u.Name, Role: gen.Role(u.Role), Status: gen.UserStatus(u.Status), MustChangePassword: u.MustChangePassword, LastLoginAt: nullableValue(u.LastLoginAt), CreatedAt: u.CreatedAt}
}

func apiKeyDTO(k domain.APIKey) gen.ApiKey {
	scopes := make([]gen.ApiKeyScope, len(k.Scopes))
	for i, s := range k.Scopes {
		scopes[i] = gen.ApiKeyScope(s)
	}
	return gen.ApiKey{Id: k.ID, Name: k.Name, Prefix: k.Prefix, Scopes: scopes, CreatedBy: k.CreatedBy, LastUsedAt: nullableValue(k.LastUsedAt), RevokedAt: nullableValue(k.RevokedAt), CreatedAt: k.CreatedAt}
}

func nullableValue[T any](value *T) nullable.Nullable[T] {
	var result nullable.Nullable[T]
	if value == nil {
		result.SetNull()
	} else {
		result.Set(*value)
	}
	return result
}

func pageRequest(limit *gen.Limit, cursor *gen.Cursor) inbound.PageRequest {
	r := inbound.PageRequest{}
	if limit != nil {
		r.Limit = *limit
	}
	if cursor != nil {
		r.Cursor = *cursor
	}
	return r
}
