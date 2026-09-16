package http

import (
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"github.com/oapi-codegen/nullable"
)

type failureDTO = struct {
	Code    gen.ProblemCode `json:"code"`
	Message string          `json:"message"`
}

func failureValue(f *domain.ProviderFailure) nullable.Nullable[failureDTO] {
	if f == nil {
		return nullableValue[failureDTO](nil)
	}
	return nullableValue(&failureDTO{Code: gen.ProblemCode(f.Code), Message: f.Message})
}

func providerDTO(p domain.Provider) gen.Provider {
	return gen.Provider{Id: p.ID, Name: p.Name, Kind: gen.ProviderKind(p.Kind), BaseUrl: nullableValue(p.BaseURL), DefaultModel: nullableValue(p.DefaultModel), ApiKeyHint: nullableValue(p.APIKeyHint), Status: gen.ConnectionStatus(p.Status), LastError: failureValue(p.LastError), LastCheckedAt: nullableValue(p.LastCheckedAt), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func agentDTO(view domain.AgentView) gen.Agent {
	a := view.Agent
	var temperature *float32
	if a.Temperature != nil {
		v := float32(*a.Temperature)
		temperature = &v
	}
	return gen.Agent{Id: a.ID, Name: a.Name, Description: a.Description, ProviderId: a.ProviderID, Model: a.Model, SystemPrompt: a.SystemPrompt, Temperature: nullableValue(temperature), MaxOutputTokens: nullableValue(a.MaxOutputTokens), ContextWindowTokens: a.ContextWindowTokens, MaxIterations: a.MaxIterations, TimeoutSeconds: a.TimeoutSeconds, CreatedBy: a.CreatedBy, ArchivedAt: nullableValue(a.ArchivedAt), CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, Ready: view.Ready, ReadinessError: failureValue(view.ReadinessError)}
}

func valuePointer[T any](value nullable.Nullable[T]) *T {
	if !value.IsSpecified() || value.IsNull() {
		return nil
	}
	v, _ := value.Get()
	return &v
}

func changeValue[T any](value nullable.Nullable[T]) domain.Change[T] {
	return domain.Change[T]{Set: value.IsSpecified(), Value: valuePointer(value)}
}

func temperatureValue(value nullable.Nullable[float32]) *float64 {
	if v := valuePointer(value); v != nil {
		n := float64(*v)
		return &n
	}
	return nil
}
