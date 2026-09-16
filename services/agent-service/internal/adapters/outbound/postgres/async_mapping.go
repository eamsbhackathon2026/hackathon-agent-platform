package postgres

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
)

func webhookDeliveryModel(v sqlcgen.WebhookDelivery) domain.WebhookDelivery {
	var statusCode *int
	if v.LastStatusCode.Valid {
		value := int(v.LastStatusCode.Int32)
		statusCode = &value
	}
	return domain.WebhookDelivery{
		ID: uuid.UUID(v.ID.Bytes), RunID: uuid.UUID(v.RunID.Bytes), APIKeyID: uuid.UUID(v.ApiKeyID.Bytes),
		URL: v.Url, Event: domain.WebhookEvent(v.Event), Payload: v.Payload,
		Status: domain.WebhookDeliveryStatus(v.Status), Attempts: int(v.Attempts),
		NextAttemptAt: timePointer(v.NextAttemptAt), LockedUntil: timePointer(v.LockedUntil),
		LastStatusCode: statusCode, LastError: stringPointer(v.LastError),
		DeliveredAt: timePointer(v.DeliveredAt), CreatedAt: v.CreatedAt.Time.UTC(),
	}
}

func int4Value(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	var value pgtype.Int4
	if value.Scan(*v) != nil {
		return pgtype.Int4{}
	}
	return value
}

func idempotencyModel(v sqlcgen.IdempotencyKey) domain.IdempotencyRecord {
	return domain.IdempotencyRecord{PrincipalKind: domain.PrincipalKind(v.PrincipalKind), PrincipalID: uuid.UUID(v.PrincipalID.Bytes), Key: v.Key, RequestHash: append([]byte(nil), v.RequestHash...), RunID: uuid.UUID(v.RunID.Bytes), ResponseStatus: int(v.ResponseStatus)}
}
