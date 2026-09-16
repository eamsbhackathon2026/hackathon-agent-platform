package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
)

func dbID(id uuid.UUID) pgtype.UUID         { return pgtype.UUID{Bytes: id, Valid: true} }
func dbTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
func optionalTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return dbTime(*t)
}
func optionalID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return dbID(*id)
}
func timePointer(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
func textValue[T ~string](s *T) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*s), Valid: true}
}
func page(p domain.PageOptions) (int32, pgtype.Timestamptz, pgtype.UUID) {
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 101 {
		limit = 101
	}
	if p.Before == nil {
		return int32(limit), pgtype.Timestamptz{}, pgtype.UUID{}
	}
	return int32(limit), dbTime(p.Before.CreatedAt), dbID(p.Before.ID)
}
func userModel(v sqlcgen.User) domain.User {
	return domain.User{ID: uuid.UUID(v.ID.Bytes), Email: v.Email, Name: v.Name, PasswordHash: v.PasswordHash, Role: domain.Role(v.Role), Status: domain.UserStatus(v.Status), MustChangePassword: v.MustChangePassword, LastLoginAt: timePointer(v.LastLoginAt), CreatedAt: v.CreatedAt.Time}
}
func refreshModel(v sqlcgen.RefreshToken) domain.RefreshToken {
	var replaced *uuid.UUID
	if v.ReplacedBy.Valid {
		id := uuid.UUID(v.ReplacedBy.Bytes)
		replaced = &id
	}
	return domain.RefreshToken{ID: uuid.UUID(v.ID.Bytes), UserID: uuid.UUID(v.UserID.Bytes), FamilyID: uuid.UUID(v.FamilyID.Bytes), TokenHash: v.TokenHash, ExpiresAt: v.ExpiresAt.Time, RevokedAt: timePointer(v.RevokedAt), ReplacedBy: replaced, CreatedAt: v.CreatedAt.Time}
}
func apiKeyModel(v sqlcgen.ApiKey) domain.APIKey {
	return domain.APIKey{ID: uuid.UUID(v.ID.Bytes), Name: v.Name, Prefix: v.Prefix, KeyHash: v.KeyHash, Scopes: v.Scopes, WebhookSecretCiphertext: v.WebhookSecretCiphertext, CreatedBy: uuid.UUID(v.CreatedBy.Bytes), LastUsedAt: timePointer(v.LastUsedAt), RevokedAt: timePointer(v.RevokedAt), CreatedAt: v.CreatedAt.Time}
}
