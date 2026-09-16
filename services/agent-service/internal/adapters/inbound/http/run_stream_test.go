package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	runservice "agent-platform/services/agent-service/internal/core/services/runs"
	"agent-platform/services/agent-service/internal/platform/clock"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

func TestRunStreamFlushesDeltaBeforeCompletion(t *testing.T) {
	release := make(chan struct{})
	service, store, principal, command, llm := streamFixture(t)
	llm.StreamFunc = func(_ context.Context, _ domain.LLMRequest, emit func(domain.LLMDelta)) (domain.LLMResult, error) {
		emit(domain.LLMDelta{Text: "A"})
		<-release
		return domain.LLMResult{Text: "AB", FinishReason: "stop"}, nil
	}
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, request *stdhttp.Request) {
		response := &runStreamResponse{ctx: request.Context(), principal: principal, command: command, runs: service}
		_ = response.VisitStreamRunResponse(w)
	}))
	defer server.Close()
	response, err := server.Client().Post(server.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" || response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("status=%d headers=%v", response.StatusCode, response.Header)
	}
	reader := bufio.NewReader(response.Body)
	prefix := readUntil(t, reader, "event: message.delta\n")
	if !strings.Contains(prefix, `"text":"A"`) {
		t.Fatalf("delta frame=%q", prefix)
	}
	close(release)
	rest, err := io.ReadAll(reader)
	if err != nil || !bytes.Contains(rest, []byte("event: run.completed")) {
		t.Fatalf("rest=%q err=%v", rest, err)
	}
	runs, _ := store.ListRuns(t.Context(), outbound.RunListOptions{Limit: 10})
	if len(runs) != 1 || runs[0].Status != domain.RunSucceeded {
		t.Fatalf("runs=%+v", runs)
	}
}

func TestRunStreamClientDisconnectCancelsDurableRun(t *testing.T) {
	service, store, principal, command, llm := streamFixture(t)
	llm.StreamFunc = func(ctx context.Context, _ domain.LLMRequest, emit func(domain.LLMDelta)) (domain.LLMResult, error) {
		emit(domain.LLMDelta{Text: "partial"})
		<-ctx.Done()
		return domain.LLMResult{}, ctx.Err()
	}
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, request *stdhttp.Request) {
		response := &runStreamResponse{ctx: request.Context(), principal: principal, command: command, runs: service}
		_ = response.VisitStreamRunResponse(w)
	}))
	defer server.Close()
	response, err := server.Client().Post(server.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	readUntil(t, reader, "event: message.delta\n")
	_ = response.Body.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := store.ListRuns(t.Context(), outbound.RunListOptions{Limit: 10})
		if len(runs) == 1 && runs[0].Status == domain.RunCancelled {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("disconnected stream did not persist cancellation")
}

func TestRunStreamReturnsProblemBeforeCommittingSSE(t *testing.T) {
	service, store, principal, command, _ := streamFixture(t)
	service.Skills = &fakes.SkillResolver{Err: domain.Invalid("skill_ids", "Invalid skill snapshot.")}
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, request *stdhttp.Request) {
		response := &runStreamResponse{ctx: request.Context(), principal: principal, command: command, runs: service}
		if err := response.VisitStreamRunResponse(w); err != nil {
			writeDomainError(w, err)
		}
	}))
	defer server.Close()

	response, err := server.Client().Post(server.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, readErr := io.ReadAll(response.Body)
	if readErr != nil || response.StatusCode != stdhttp.StatusBadRequest || response.Header.Get("Content-Type") != "application/problem+json" || bytes.Contains(body, []byte("event:")) {
		t.Fatalf("status=%d headers=%v body=%s err=%v", response.StatusCode, response.Header, body, readErr)
	}
	runs, _ := store.ListRuns(t.Context(), outbound.RunListOptions{Limit: 10})
	if len(runs) != 0 {
		t.Fatalf("pre-stream failure persisted runs=%+v", runs)
	}
}

func TestSSEHeartbeatAndPublicEventsExcludeProviderMetadata(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := newSSEWriter(recorder)
	if err := writer.heartbeat(t.Context()); err != nil || recorder.Body.String() != ": ping\n\n" {
		t.Fatalf("heartbeat=%q err=%v", recorder.Body.String(), err)
	}
	message := domain.Message{ID: uuid.New(), SessionID: uuid.New(), Role: "assistant", ToolCalls: []domain.ToolCall{{ID: "call", Name: "tool", Arguments: json.RawMessage(`{}`), ProviderMeta: []byte(`{"secret":"call"}`)}}, ProviderMeta: []byte(`{"secret":"message"}`), CreatedAt: time.Now()}
	event, err := eventDTO(domain.RunEvent{Type: domain.EventMessageCompleted, Message: &message})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(event)
	if bytes.Contains(raw, []byte("secret")) || bytes.Contains(raw, []byte("provider_meta")) {
		t.Fatalf("public event leaked private metadata: %s", raw)
	}
}

func streamFixture(t *testing.T) (*runservice.Service, *fakes.RunStore, domain.Principal, inbound.RunCommand, *fakes.ScriptedLLM) {
	t.Helper()
	now := time.Now().UTC()
	providerID, agentID, userID := uuid.New(), uuid.New(), uuid.New()
	cipher := fakes.Cipher{}
	secret, _ := cipher.Encrypt([]byte("secret"), []byte("provider:"+providerID.String()))
	catalog := fakes.NewCatalogStore()
	if err := catalog.CreateProvider(t.Context(), domain.Provider{ID: providerID, Name: "Provider", Kind: domain.ProviderGemini, APIKeyCiphertext: secret, Status: domain.ConnectionOK, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.CreateAgent(t.Context(), domain.Agent{ID: agentID, Name: "Agent", ProviderID: providerID, Model: "model", MaxIterations: 8, TimeoutSeconds: 120, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	store, llm, tools := fakes.NewRunStore(), &fakes.ScriptedLLM{}, &fakes.ToolSet{}
	service, err := runservice.NewService(runservice.Dependencies{Agents: catalog, Providers: catalog, Sessions: store, Messages: store, Snapshots: store, Runs: store, Spans: store, Tx: store, Cipher: cipher, Factory: &fakes.FakeFactory{Client: llm}, Tools: fakes.ToolResolver{Set: tools}, Skills: &fakes.SkillResolver{}, Clock: clock.RealClock{}, IDs: clock.UUIDv7IDGenerator{}, CancelPoll: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	principal := domain.Principal{Kind: domain.PrincipalUser, UserID: userID, Role: domain.RoleMember}
	command := inbound.RunCommand{AgentID: agentID, Input: "Hi", Mode: domain.RunModeStream, Metadata: json.RawMessage(`{}`)}
	return service, store, principal, command, llm
}

func readUntil(t *testing.T, reader *bufio.Reader, marker string) string {
	t.Helper()
	var result strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		result.WriteString(line)
		if line == marker {
			data, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			result.WriteString(data)
			return result.String()
		}
	}
}
