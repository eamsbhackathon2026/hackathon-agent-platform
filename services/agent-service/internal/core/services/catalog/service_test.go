package catalog_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/services/catalog"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"github.com/google/uuid"
)

type urlPolicy struct{}

func (urlPolicy) ValidateURL(s string) error {
	if s == "https://blocked.example" {
		return domain.ErrEgressDenied
	}
	return nil
}
func ptr[T any](v T) *T { return &v }

type fixture struct {
	s       *catalog.Service
	store   *fakes.CatalogStore
	llm     *fakes.ScriptedLLM
	factory *fakes.FakeFactory
	p       domain.Principal
}

func setup(t *testing.T) fixture {
	t.Helper()
	store := fakes.NewCatalogStore()
	llm := &fakes.ScriptedLLM{Models: []string{"z", "a", "a", "m"}}
	factory := &fakes.FakeFactory{Client: llm}
	s, err := catalog.NewService(catalog.Dependencies{Providers: store, Agents: store, Tx: store, Cipher: fakes.Cipher{}, Factory: factory, URLs: urlPolicy{}, Clock: fakes.Clock{Time: time.Date(2026, 9, 14, 1, 0, 0, 0, time.UTC)}, IDs: fakes.IDs{}})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{s, store, llm, factory, domain.Principal{Kind: domain.PrincipalUser, Role: domain.RoleAdmin, UserID: uuid.New()}}
}
func (f fixture) provider(t *testing.T, name string) domain.Provider {
	t.Helper()
	p, err := f.s.CreateProvider(context.Background(), f.p, inbound.ProviderCreateCommand{Name: name, Kind: domain.ProviderGemini, APIKey: "secret-1234", DefaultModel: ptr("model")})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func (f fixture) agent(t *testing.T, p domain.Provider, name string) domain.AgentView {
	t.Helper()
	a, err := f.s.CreateAgent(context.Background(), f.p, inbound.AgentCreateCommand{Name: name, ProviderID: p.ID, Model: "model"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestProviderSecretPatch(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.provider(t, "AI")
	if bytes.Contains(p.APIKeyCiphertext, []byte("secret-1234")) || *p.APIKeyHint != "1234" {
		t.Fatal("plaintext persisted or wrong hint")
	}
	plain, err := fakes.Cipher{}.Decrypt(p.APIKeyCiphertext, []byte("provider:"+p.ID.String()))
	if err != nil || string(plain) != "secret-1234" {
		t.Fatal("AAD/key mismatch")
	}
	updated, err := f.s.UpdateProvider(ctx, f.p, p.ID, inbound.ProviderUpdateCommand{Name: ptr("Renamed")})
	if err != nil || !bytes.Equal(updated.APIKeyCiphertext, p.APIKeyCiphertext) {
		t.Fatal("omitted key overwritten", err)
	}
	checked, err := f.s.TestProvider(ctx, f.p, p.ID)
	if err != nil || !checked.OK {
		t.Fatal(checked, err)
	}
	updated, err = f.s.UpdateProvider(ctx, f.p, p.ID, inbound.ProviderUpdateCommand{APIKey: domain.Change[string]{Set: true, Value: ptr("mới-5678")}})
	if err != nil || *updated.APIKeyHint != "5678" || updated.LastCheckedAt != nil || updated.Status != domain.ConnectionUnchecked {
		t.Fatal(updated, err)
	}
	updated, err = f.s.UpdateProvider(ctx, f.p, p.ID, inbound.ProviderUpdateCommand{APIKey: domain.Change[string]{Set: true}})
	if err != nil || len(updated.APIKeyCiphertext) != 0 || updated.APIKeyHint != nil {
		t.Fatal(updated, err)
	}
	result, err := f.s.TestProvider(ctx, f.p, p.ID)
	if err != nil || result.OK || result.Failure.Code != "provider_not_configured" {
		t.Fatal(result, err)
	}
	_, err = f.s.UpdateProvider(ctx, f.p, p.ID, inbound.ProviderUpdateCommand{Kind: ptr(domain.ProviderOpenAICompatible)})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatal(err)
	}
	current, err := f.s.GetProvider(ctx, f.p, p.ID)
	if err != nil || current.Kind != domain.ProviderGemini {
		t.Fatal("invalid patch persisted", err)
	}
	_, err = f.s.UpdateProvider(ctx, f.p, p.ID, inbound.ProviderUpdateCommand{BaseURL: domain.Change[string]{Set: true, Value: ptr("https://blocked.example")}})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatal(err)
	}
}

func TestAgentArchiveReadinessAndRelatedConflict(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.provider(t, "AI")
	a := f.agent(t, p, "Assistant")
	if !a.Ready || a.Agent.ContextWindowTokens != domain.DefaultContextWindowTokens || a.Agent.MaxIterations != 8 || a.Agent.TimeoutSeconds != 120 {
		t.Fatal(a)
	}
	err := f.s.DeleteProvider(ctx, f.p, p.ID)
	var detail *domain.Error
	if !errors.As(err, &detail) || !errors.Is(err, domain.ErrConflict) || len(detail.RelatedAgents) != 1 || detail.RelatedAgents[0].ID != a.Agent.ID {
		t.Fatal(err)
	}
	updated, err := f.s.UpdateAgent(ctx, f.p, a.Agent.ID, inbound.AgentUpdateCommand{Name: ptr("Updated"), Description: ptr("Description"), Model: ptr("other"), SystemPrompt: ptr("Prompt"), ProviderID: &p.ID, Temperature: domain.Change[float64]{Set: true, Value: ptr(0.5)}, MaxOutputTokens: domain.Change[int]{Set: true, Value: ptr(50)}, ContextWindowTokens: ptr(65536), MaxIterations: ptr(10), TimeoutSeconds: ptr(60)})
	if err != nil || updated.Agent.Model != "other" || updated.Agent.Temperature == nil || updated.Agent.ContextWindowTokens != 65536 {
		t.Fatal(updated, err)
	}
	updated, err = f.s.UpdateAgent(ctx, f.p, a.Agent.ID, inbound.AgentUpdateCommand{Temperature: domain.Change[float64]{Set: true}, MaxOutputTokens: domain.Change[int]{Set: true}})
	if err != nil || updated.Agent.Temperature != nil || updated.Agent.MaxOutputTokens != nil {
		t.Fatal(updated, err)
	}
	_, err = f.s.UpdateAgent(ctx, f.p, a.Agent.ID, inbound.AgentUpdateCommand{MaxIterations: ptr(26)})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatal(err)
	}
	_, err = f.s.CreateAgent(ctx, f.p, inbound.AgentCreateCommand{Name: "Missing", Model: "m", ProviderID: uuid.New()})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if err = f.s.DeleteAgent(ctx, f.p, a.Agent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.GetAgent(ctx, f.p, a.Agent.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	reused := f.agent(t, p, "Updated")
	if reused.Agent.ID == a.Agent.ID {
		t.Fatal("archive ID reused")
	}
	if err = f.s.DeleteAgent(ctx, f.p, reused.Agent.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.s.DeleteProvider(ctx, f.p, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.GetProvider(ctx, f.p, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestForbiddenMember(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.provider(t, "AI")
	a := f.agent(t, p, "Agent")
	f.p.Role = domain.RoleMember
	checks := []func() error{
		func() error { _, e := f.s.CreateProvider(ctx, f.p, inbound.ProviderCreateCommand{}); return e }, func() error { _, e := f.s.UpdateProvider(ctx, f.p, p.ID, inbound.ProviderUpdateCommand{}); return e }, func() error { return f.s.DeleteProvider(ctx, f.p, p.ID) }, func() error { _, e := f.s.TestProvider(ctx, f.p, p.ID); return e }, func() error { _, e := f.s.ListProviderModels(ctx, f.p, p.ID, inbound.PageRequest{}); return e }, func() error { _, e := f.s.CreateAgent(ctx, f.p, inbound.AgentCreateCommand{}); return e }, func() error { _, e := f.s.UpdateAgent(ctx, f.p, a.Agent.ID, inbound.AgentUpdateCommand{}); return e }, func() error { return f.s.DeleteAgent(ctx, f.p, a.Agent.ID) }}
	for i, check := range checks {
		if e := check(); !errors.Is(e, domain.ErrForbidden) {
			t.Fatalf("%d: %v", i, e)
		}
	}
	if _, err := f.s.GetAgent(ctx, f.p, a.Agent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.ListProviders(ctx, f.p, inbound.PageRequest{}); err != nil {
		t.Fatal(err)
	}
}

func TestPaginationAndReadiness(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.provider(t, "AI")
	f.provider(t, "Second")
	f.agent(t, p, "A")
	f.agent(t, p, "B")
	page, err := f.s.ListProviders(ctx, f.p, inbound.PageRequest{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatal(page, err)
	}
	next, err := f.s.ListProviders(ctx, f.p, inbound.PageRequest{Limit: 1, Cursor: *page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].ID == page.Items[0].ID {
		t.Fatal(next, err)
	}
	agents, err := f.s.ListAgents(ctx, f.p, inbound.PageRequest{Limit: 1})
	if err != nil || agents.NextCursor == nil {
		t.Fatal(agents, err)
	}
	nextAgents, err := f.s.ListAgents(ctx, f.p, inbound.PageRequest{Cursor: *agents.NextCursor})
	if err != nil || len(nextAgents.Items) != 1 {
		t.Fatal(nextAgents, err)
	}
	f.llm.ModelsError = domain.ErrProviderAuth
	if _, err = f.s.TestProvider(ctx, f.p, p.ID); err != nil {
		t.Fatal(err)
	}
	agents, err = f.s.ListAgents(ctx, f.p, inbound.PageRequest{})
	if err != nil || agents.Items[0].Ready || agents.Items[0].ReadinessError.Code != "provider_auth_failed" {
		t.Fatal(agents, err)
	}
	for _, req := range []inbound.PageRequest{{Limit: 101}, {Cursor: "!"}, {Cursor: "e30"}} {
		if _, err = f.s.ListAgents(ctx, f.p, req); !errors.Is(err, domain.ErrValidation) {
			t.Fatal(err)
		}
	}
}

func TestModelPaginationFallbackAndSafeErrors(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.provider(t, "AI")
	page, err := f.s.ListProviderModels(ctx, f.p, p.ID, inbound.PageRequest{Limit: 2})
	if err != nil || fmt.Sprint(page.Items) != "[a m]" || page.NextCursor == nil {
		t.Fatal(page, err)
	}
	next, err := f.s.ListProviderModels(ctx, f.p, p.ID, inbound.PageRequest{Limit: 2, Cursor: *page.NextCursor})
	if err != nil || fmt.Sprint(next.Items) != "[z]" || next.NextCursor != nil {
		t.Fatal(next, err)
	}
	for _, req := range []inbound.PageRequest{{Limit: 101}, {Cursor: "!"}, {Cursor: "_w"}} {
		if _, err = f.s.ListProviderModels(ctx, f.p, p.ID, req); !errors.Is(err, domain.ErrValidation) {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		err  error
		code string
	}{{domain.ErrProviderAuth, "provider_auth_failed"}, {domain.ErrProviderUnreachable, "provider_unreachable"}, {domain.ErrModelNotFound, "model_not_found"}, {domain.ErrProviderRateLimited, "rate_limited"}, {domain.ErrProviderBadRequest, "validation_failed"}, {errors.New("SDK leaked key=secret"), "provider_unreachable"}} {
		f.llm.ModelsError = tc.err
		result, e := f.s.TestProvider(ctx, f.p, p.ID)
		if e != nil || result.OK || result.Failure.Code != tc.code {
			t.Fatal(result, e)
		}
		if len(f.llm.Requests()) != 0 {
			t.Fatal("unsafe replay")
		}
		_, e = f.s.ListProviderModels(ctx, f.p, p.ID, inbound.PageRequest{})
		if e == nil || e.Error() == "SDK leaked key=secret" {
			t.Fatal(e)
		}
	}
	f.llm.ModelsError = domain.ErrModelsUnsupported
	result, err := f.s.TestProvider(ctx, f.p, p.ID)
	if err != nil || !result.OK || len(f.llm.Requests()) != 1 {
		t.Fatal(result, err)
	}
	req := f.llm.Requests()[0]
	if *req.MaxOutputTokens != 1 || len(req.Tools) != 0 || req.Model != "model" {
		t.Fatal(req)
	}
	_, err = f.s.UpdateProvider(ctx, f.p, p.ID, inbound.ProviderUpdateCommand{DefaultModel: domain.Change[string]{Set: true}})
	if err != nil {
		t.Fatal(err)
	}
	result, err = f.s.TestProvider(ctx, f.p, p.ID)
	if err != nil || result.Failure.Code != "not_implemented" || len(f.llm.Requests()) != 1 {
		t.Fatal(result, err)
	}
}

func TestStaleCheckCannotOverwriteEditedConfig(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.provider(t, "AI")
	started := make(chan struct{})
	release := make(chan struct{})
	f.llm.ModelFunc = func(context.Context) ([]string, error) { close(started); <-release; return nil, domain.ErrProviderAuth }
	done := make(chan error, 1)
	go func() { _, err := f.s.TestProvider(ctx, f.p, p.ID); done <- err }()
	<-started
	_, err := f.s.UpdateProvider(ctx, f.p, p.ID, inbound.ProviderUpdateCommand{APIKey: domain.Change[string]{Set: true, Value: ptr("new-key")}})
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	if err = <-done; !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	current, err := f.s.GetProvider(ctx, f.p, p.ID)
	if err != nil || current.Status != domain.ConnectionUnchecked || current.LastError != nil {
		t.Fatal(current, err)
	}
}

func TestConstructorRequiresDependencies(t *testing.T) {
	if _, err := catalog.NewService(catalog.Dependencies{}); err == nil {
		t.Fatal("missing dependencies accepted")
	}
}
