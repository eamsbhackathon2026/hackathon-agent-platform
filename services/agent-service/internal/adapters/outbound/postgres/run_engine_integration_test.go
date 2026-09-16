//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	runservice "agent-platform/services/agent-service/internal/core/services/runs"
	"agent-platform/services/agent-service/internal/platform/clock"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"agent-platform/services/agent-service/internal/testsupport/pgtest"
)

func TestProviderMetadataRoundTripsThroughPostgresIntoNextRun(t *testing.T) {
	ctx := t.Context()
	store := postgres.NewStore(pgtest.NewPool(t))
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID, providerID, agentID := uuid.New(), uuid.New(), uuid.New()
	if err := store.CreateUser(ctx, domain.User{ID: userID, Email: "run@example.test", Name: "Runner", PasswordHash: "hash", Role: domain.RoleMember, Status: domain.UserActive, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	cipher := fakes.Cipher{}
	secret, _ := cipher.Encrypt([]byte("secret"), []byte("provider:"+providerID.String()))
	if err := store.CreateProvider(ctx, domain.Provider{ID: providerID, Name: "Run provider", Kind: domain.ProviderGemini, APIKeyCiphertext: secret, Status: domain.ConnectionOK, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, domain.Agent{ID: agentID, Name: "Run agent", ProviderID: providerID, Model: "model", MaxIterations: 8, TimeoutSeconds: 120, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	messageMeta := json.RawMessage(`{"parts":[{"text":"","thought_signature":"opaque"},{"function_call":{"id":"call-1"}}]}`)
	callMeta := json.RawMessage(`{"call":{"id":"call-1"},"signature":"private"}`)
	llm := &fakes.ScriptedLLM{Steps: []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "lookup", Arguments: json.RawMessage(`{"q":"one"}`), ProviderMeta: callMeta}}, ProviderMeta: messageMeta, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Đã xong", FinishReason: "stop"}},
		{Result: domain.LLMResult{Text: "Tiếp tục", FinishReason: "stop"}},
	}}
	tools := &fakes.ToolSet{Results: []domain.ToolResult{{Content: "result"}}}
	service, err := runservice.NewService(runservice.Dependencies{Agents: store, Providers: store, Sessions: store, Messages: store, Snapshots: store, Runs: store, Spans: store, Tx: store, Cipher: cipher, Factory: &fakes.FakeFactory{Client: llm}, Tools: fakes.ToolResolver{Set: tools}, Skills: &fakes.SkillResolver{}, Clock: clock.RealClock{}, IDs: clock.UUIDv7IDGenerator{}})
	if err != nil {
		t.Fatal(err)
	}
	principal := domain.Principal{Kind: domain.PrincipalUser, UserID: userID, Role: domain.RoleMember}
	first, err := service.RunSync(ctx, principal, inbound.RunCommand{AgentID: agentID, Input: "Lượt một", Mode: domain.RunModeSync, Metadata: json.RawMessage(`{}`)})
	if err != nil || first.Status != domain.RunSucceeded {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := service.RunSync(ctx, principal, inbound.RunCommand{AgentID: agentID, Input: "Lượt hai", SessionID: &first.SessionID, Mode: domain.RunModeSync, Metadata: json.RawMessage(`{}`)})
	if err != nil || second.Status != domain.RunSucceeded {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	requests := llm.Requests()
	if len(requests) != 3 || len(requests[2].Messages) < 5 {
		t.Fatalf("requests=%d history=%d", len(requests), len(requests[2].Messages))
	}
	assistant := requests[2].Messages[1]
	if !sameJSON(assistant.ProviderMeta, messageMeta) || len(assistant.ToolCalls) != 1 || !sameJSON(assistant.ToolCalls[0].ProviderMeta, callMeta) {
		t.Fatal("Postgres round-trip lost private provider metadata")
	}
	messages, err := store.ListRecentMessages(ctx, first.SessionID, 50)
	if err != nil || len(messages) != 6 || messages[1].Seq != 2 || messages[5].Seq != 6 {
		t.Fatalf("messages=%d err=%v", len(messages), err)
	}
}

func TestConversationSerializesActiveRuns(t *testing.T) {
	tests := []struct {
		name               string
		useSessionKey      bool
		createSessionFirst bool
		secondLockCall     int
	}{
		{name: "session id", secondLockCall: 2, createSessionFirst: true},
		{name: "existing session key", useSessionKey: true, secondLockCall: 3, createSessionFirst: true},
		{name: "new session key", useSessionKey: true, secondLockCall: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testConversationSerializesActiveRuns(t, test.useSessionKey, test.createSessionFirst, test.secondLockCall)
		})
	}
}

func testConversationSerializesActiveRuns(t *testing.T, useSessionKey, createSessionFirst bool, secondLockCall int) {
	ctx := t.Context()
	store := postgres.NewStore(pgtest.NewPool(t))
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID, keyID, providerID, agentID, sessionID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := store.CreateUser(ctx, domain.User{ID: userID, Email: "serial-run-" + uuid.NewString() + "@example.test", Name: "Runner", PasswordHash: "hash", Role: domain.RoleMember, Status: domain.UserActive, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAPIKey(ctx, domain.APIKey{ID: keyID, Name: "Serial key", Prefix: uuid.NewString(), KeyHash: []byte("hash"), Scopes: []string{"runs:write"}, WebhookSecretCiphertext: []byte("secret"), CreatedBy: userID, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	cipher := fakes.Cipher{}
	secret, _ := cipher.Encrypt([]byte("secret"), []byte("provider:"+providerID.String()))
	if err := store.CreateProvider(ctx, domain.Provider{ID: providerID, Name: "Serial provider", Kind: domain.ProviderGemini, APIKeyCiphertext: secret, Status: domain.ConnectionOK, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, domain.Agent{ID: agentID, Name: "Serial agent", ProviderID: providerID, Model: "model", MaxIterations: 8, TimeoutSeconds: 120, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	principal := domain.Principal{Kind: domain.PrincipalUser, UserID: userID, Role: domain.RoleMember}
	command := func(input string) inbound.RunCommand {
		return inbound.RunCommand{AgentID: agentID, Input: input, SessionID: &sessionID, Metadata: json.RawMessage(`{}`)}
	}
	if useSessionKey {
		sessionKey := "serial-" + uuid.NewString()
		principal = domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyID, Scopes: []string{"runs:write"}}
		command = func(input string) inbound.RunCommand {
			return inbound.RunCommand{AgentID: agentID, Input: input, SessionKey: &sessionKey, Metadata: json.RawMessage(`{}`)}
		}
		if createSessionFirst {
			if _, err := store.CreateSession(ctx, domain.Session{ID: sessionID, AgentID: agentID, Source: domain.RunSourceAPI, CreatedByAPIKeyID: &keyID, ExternalKey: &sessionKey, Title: "Conversation", CreatedAt: now, UpdatedAt: now}); err != nil {
				t.Fatal(err)
			}
		}
	} else if _, err := store.CreateSession(ctx, domain.Session{ID: sessionID, AgentID: agentID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Conversation", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	firstLLMStarted, releaseLLM := make(chan struct{}), make(chan struct{})
	var startedOnce sync.Once
	llm := &fakes.ScriptedLLM{StreamFunc: func(callCtx context.Context, _ domain.LLMRequest, _ func(domain.LLMDelta)) (domain.LLMResult, error) {
		startedOnce.Do(func() { close(firstLLMStarted) })
		select {
		case <-releaseLLM:
			return domain.LLMResult{Text: "done", FinishReason: "stop"}, nil
		case <-callCtx.Done():
			return domain.LLMResult{}, callCtx.Err()
		}
	}}
	sessions := newObservedSessionRepository(store, secondLockCall)
	service, err := runservice.NewService(runservice.Dependencies{Agents: store, Providers: store, Sessions: sessions, Messages: store, Snapshots: store, Runs: store, Spans: store, Tx: store, Cipher: cipher, Factory: &fakes.FakeFactory{Client: llm}, Tools: fakes.ToolResolver{Set: &fakes.ToolSet{}}, Skills: &fakes.SkillResolver{}, Clock: clock.RealClock{}, IDs: clock.UUIDv7IDGenerator{}})
	if err != nil {
		t.Fatal(err)
	}
	type runResult struct {
		run domain.Run
		err error
	}
	firstResult := make(chan runResult, 1)
	go func() {
		run, runErr := service.RunSync(ctx, principal, command("first"))
		firstResult <- runResult{run: run, err: runErr}
	}()
	<-sessions.firstAtGuard

	secondCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	secondResult := make(chan error, 1)
	go func() {
		_, runErr := service.RunSync(secondCtx, principal, command("second"))
		secondResult <- runErr
	}()
	<-sessions.secondLockAttempt
	select {
	case <-sessions.secondAtGuard:
		t.Fatal("second run passed the session lock before the first transaction committed")
	case <-time.After(100 * time.Millisecond):
	}
	close(sessions.releaseFirstGuard)
	<-firstLLMStarted
	secondErr := <-secondResult
	if !errors.Is(secondErr, domain.ErrRunInProgress) {
		t.Fatalf("second run err=%v", secondErr)
	}
	close(releaseLLM)
	first := <-firstResult
	if first.err != nil {
		t.Fatalf("first run err=%v", first.err)
	}
	messages, err := store.ListRecentMessages(ctx, first.run.SessionID, 20)
	if err != nil || len(messages) != 2 || messages[0].Content != "first" || messages[1].Role != "assistant" {
		t.Fatalf("messages=%+v err=%v", messages, err)
	}
}

type observedSessionRepository struct {
	outbound.SessionRepository
	mu                               sync.Mutex
	lockCalls, guardCalls            int
	secondLockCall                   int
	firstAtGuard, releaseFirstGuard  chan struct{}
	secondLockAttempt, secondAtGuard chan struct{}
}

func newObservedSessionRepository(repository outbound.SessionRepository, secondLockCall int) *observedSessionRepository {
	return &observedSessionRepository{
		SessionRepository: repository,
		secondLockCall:    secondLockCall,
		firstAtGuard:      make(chan struct{}),
		releaseFirstGuard: make(chan struct{}),
		secondLockAttempt: make(chan struct{}),
		secondAtGuard:     make(chan struct{}),
	}
}

func (r *observedSessionRepository) GetSessionForUpdate(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	r.observeLockAttempt()
	return r.SessionRepository.GetSessionForUpdate(ctx, id)
}

func (r *observedSessionRepository) GetSessionByExternalKey(ctx context.Context, keyID uuid.UUID, externalKey string) (domain.Session, error) {
	r.observeLockAttempt()
	return r.SessionRepository.GetSessionByExternalKey(ctx, keyID, externalKey)
}

func (r *observedSessionRepository) observeLockAttempt() {
	r.mu.Lock()
	r.lockCalls++
	call := r.lockCalls
	r.mu.Unlock()
	if call == r.secondLockCall {
		close(r.secondLockAttempt)
	}
}

func (r *observedSessionRepository) HasActiveRuns(ctx context.Context, id uuid.UUID) (bool, error) {
	r.mu.Lock()
	r.guardCalls++
	call := r.guardCalls
	r.mu.Unlock()
	if call == 1 {
		close(r.firstAtGuard)
		select {
		case <-r.releaseFirstGuard:
		case <-ctx.Done():
			return false, ctx.Err()
		}
	} else if call == 2 {
		close(r.secondAtGuard)
	}
	return r.SessionRepository.HasActiveRuns(ctx, id)
}

func sameJSON(a, b []byte) bool {
	var left, right any
	return json.Unmarshal(a, &left) == nil && json.Unmarshal(b, &right) == nil && reflect.DeepEqual(left, right)
}
