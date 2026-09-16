//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	runservice "agent-platform/services/agent-service/internal/core/services/runs"
	"agent-platform/services/agent-service/internal/platform/clock"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"agent-platform/services/agent-service/internal/testsupport/pgtest"
)

func TestRunPanicTerminalizesRunAndRootSpanAtomically(t *testing.T) {
	ctx := t.Context()
	store := postgres.NewStore(pgtest.NewPool(t))
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID, providerID, agentID := uuid.New(), uuid.New(), uuid.New()
	if err := store.CreateUser(ctx, domain.User{ID: userID, Email: "panic-run@example.test", Name: "Runner", PasswordHash: "hash", Role: domain.RoleMember, Status: domain.UserActive, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	cipher := fakes.Cipher{}
	secret, err := cipher.Encrypt([]byte("secret"), []byte("provider:"+providerID.String()))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CreateProvider(ctx, domain.Provider{ID: providerID, Name: "Panic provider", Kind: domain.ProviderGemini, APIKeyCiphertext: secret, Status: domain.ConnectionOK, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err = store.CreateAgent(ctx, domain.Agent{ID: agentID, Name: "Panic agent", ProviderID: providerID, Model: "model", MaxIterations: 4, TimeoutSeconds: 30, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	llm := &fakes.ScriptedLLM{StreamFunc: func(context.Context, domain.LLMRequest, func(domain.LLMDelta)) (domain.LLMResult, error) {
		panic("provider panic must not escape")
	}}
	service, err := runservice.NewService(runservice.Dependencies{Agents: store, Providers: store, Sessions: store, Messages: store, Snapshots: store, Runs: store, Spans: store, Tx: store, Cipher: cipher, Factory: &fakes.FakeFactory{Client: llm}, Tools: fakes.ToolResolver{Set: &fakes.ToolSet{}}, Skills: &fakes.SkillResolver{}, Clock: clock.RealClock{}, IDs: clock.UUIDv7IDGenerator{}})
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.RunSync(ctx, domain.Principal{Kind: domain.PrincipalUser, UserID: userID, Role: domain.RoleMember}, inbound.RunCommand{AgentID: agentID, Input: "panic", Mode: domain.RunModeSync, Metadata: json.RawMessage(`{}`)})
	if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "internal" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	persisted, err := store.GetRun(ctx, run.ID)
	if err != nil || persisted.Status != domain.RunFailed || persisted.Failure == nil || persisted.Failure.Code != "internal" {
		t.Fatalf("persisted=%+v err=%v", persisted, err)
	}
	spans, err := store.ListSpans(ctx, run.ID, 20, nil)
	if err != nil || len(spans) != 1 || spans[0].Kind != domain.SpanRun || spans[0].Status != domain.SpanError {
		t.Fatalf("spans=%+v err=%v", spans, err)
	}
}
