package domain_test

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"errors"
	"github.com/google/uuid"
	"math"
	"strings"
	"testing"
)

func value[T any](v T) *T { return &v }
func TestCatalogValidation(t *testing.T) {
	for _, p := range []domain.Provider{{Name: "ok", Kind: domain.ProviderGemini}, {Name: "GreenNode", Kind: domain.ProviderGreenNode}, {Name: strings.Repeat("á", 200), Kind: domain.ProviderOpenAICompatible, BaseURL: value("https://example.com"), DefaultModel: value("")}} {
		if err := domain.ValidateProvider(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []domain.Provider{{Name: "", Kind: domain.ProviderGemini}, {Name: "ok", Kind: "other"}, {Name: "ok", Kind: domain.ProviderOpenAICompatible}, {Name: "ok", Kind: domain.ProviderGreenNode, BaseURL: value("https://example.com")}, {Name: "ok", Kind: domain.ProviderGemini, BaseURL: value("")}, {Name: "ok", Kind: domain.ProviderGemini, DefaultModel: value(strings.Repeat("a", 201))}} {
		if err := domain.ValidateProvider(p); !errors.Is(err, domain.ErrValidation) {
			t.Fatal(p, err)
		}
	}
	if err := domain.ValidateProviderKey(strings.Repeat("á", 8192)); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "  ", strings.Repeat("a", 8193), string([]byte{255})} {
		if err := domain.ValidateProviderKey(key); !errors.Is(err, domain.ErrValidation) {
			t.Fatal(err)
		}
	}
	good := domain.Agent{Name: "ok", ProviderID: uuid.New(), Model: "model", ContextWindowTokens: domain.DefaultContextWindowTokens, MaxIterations: 8, TimeoutSeconds: 120}
	if err := domain.ValidateAgent(good); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*domain.Agent){func(a *domain.Agent) { a.Name = "" }, func(a *domain.Agent) { a.Description = strings.Repeat("a", 4001) }, func(a *domain.Agent) { a.Model = "" }, func(a *domain.Agent) { a.SystemPrompt = strings.Repeat("a", 100001) }, func(a *domain.Agent) { a.ProviderID = uuid.Nil }, func(a *domain.Agent) { a.Temperature = value(-0.1) }, func(a *domain.Agent) { a.Temperature = value(math.NaN()) }, func(a *domain.Agent) { a.Temperature = value(math.Inf(1)) }, func(a *domain.Agent) { a.Temperature = value(2.1) }, func(a *domain.Agent) { a.MaxOutputTokens = value(0) }, func(a *domain.Agent) { a.ContextWindowTokens = 8191 }, func(a *domain.Agent) { a.ContextWindowTokens = 2000001 }, func(a *domain.Agent) { a.MaxOutputTokens = value(a.ContextWindowTokens) }, func(a *domain.Agent) { a.MaxIterations = 0 }, func(a *domain.Agent) { a.MaxIterations = 26 }, func(a *domain.Agent) { a.TimeoutSeconds = 9 }, func(a *domain.Agent) { a.TimeoutSeconds = 601 }} {
		a := good
		mutate(&a)
		if err := domain.ValidateAgent(a); !errors.Is(err, domain.ErrValidation) {
			t.Fatal(a, err)
		}
	}
	good.Temperature = value(2.0)
	good.ContextWindowTokens = domain.MinContextWindowTokens
	good.MaxOutputTokens = value(domain.MaxOutputTokensForContext(good.ContextWindowTokens))
	if err := domain.ValidateAgent(good); err != nil {
		t.Fatal(err)
	}
	good.MaxOutputTokens = value(domain.MaxOutputTokensForContext(good.ContextWindowTokens) + 1)
	if err := domain.ValidateAgent(good); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unsafe context budget accepted: %v", err)
	}
}
