package postgres

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type providerFailureJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func failureJSON(v *domain.ProviderFailure) []byte {
	if v == nil {
		return nil
	}
	// A struct containing only strings cannot fail JSON encoding.
	b, _ := json.Marshal(providerFailureJSON{Code: v.Code, Message: v.Message})
	return b
}
func stringPointer(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
func catalogTime(t time.Time) pgtype.Timestamptz { return dbTime(t.UTC().Truncate(time.Microsecond)) }
func catalogTimePointer(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}
func providerModel(v sqlcgen.LlmProvider) (domain.Provider, error) {
	p := domain.Provider{ID: uuid.UUID(v.ID.Bytes), Name: v.Name, Kind: domain.ProviderKind(v.Kind), BaseURL: stringPointer(v.BaseUrl), APIKeyCiphertext: v.ApiKeyCiphertext, APIKeyHint: stringPointer(v.ApiKeyHint), DefaultModel: stringPointer(v.DefaultModel), Status: domain.ConnectionStatus(v.Status), LastCheckedAt: catalogTimePointer(v.LastCheckedAt), CreatedAt: v.CreatedAt.Time.UTC(), UpdatedAt: v.UpdatedAt.Time.UTC(), Revision: v.Revision}
	if len(v.LastError) > 0 {
		var failure providerFailureJSON
		if err := json.Unmarshal(v.LastError, &failure); err != nil {
			return domain.Provider{}, fmt.Errorf("decode provider failure: %w", err)
		}
		p.LastError = &domain.ProviderFailure{Code: failure.Code, Message: failure.Message}
	}
	return p, nil
}
func agentProviderID(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{}
	}
	return dbID(id)
}
func floatValue(v *float64) pgtype.Float8 {
	if v == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *v, Valid: true}
}
func intValue(v *int) (pgtype.Int4, error) {
	if v == nil {
		return pgtype.Int4{}, nil
	}
	if *v < 1 || *v > math.MaxInt32 {
		return pgtype.Int4{}, domain.ErrValidation
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}, nil
}
func agentModel(v sqlcgen.Agent) domain.Agent {
	a := domain.Agent{ID: uuid.UUID(v.ID.Bytes), Name: v.Name, Description: v.Description, ProviderID: uuid.UUID(v.ProviderID.Bytes), Model: v.Model, SystemPrompt: v.SystemPrompt, ContextWindowTokens: int(v.ContextWindowTokens), MaxIterations: int(v.MaxIterations), TimeoutSeconds: int(v.TimeoutSeconds), CreatedBy: uuid.UUID(v.CreatedBy.Bytes), ArchivedAt: catalogTimePointer(v.ArchivedAt), CreatedAt: v.CreatedAt.Time.UTC(), UpdatedAt: v.UpdatedAt.Time.UTC()}
	if v.Temperature.Valid {
		a.Temperature = &v.Temperature.Float64
	}
	a.ShowThinking = v.ShowThinking
	if v.MaxOutputTokens.Valid {
		n := int(v.MaxOutputTokens.Int32)
		a.MaxOutputTokens = &n
	}
	return a
}
