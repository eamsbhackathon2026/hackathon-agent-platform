package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

type engineFixture struct {
	service   *Service
	store     *fakes.RunStore
	llm       *fakes.ScriptedLLM
	tools     *fakes.ToolSet
	principal domain.Principal
	command   inbound.RunCommand
	agent     domain.Agent
}

func newEngineFixture(t *testing.T, steps []fakes.LLMStep, configure func(*domain.Agent), tools *fakes.ToolSet) engineFixture {
	t.Helper()
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	providerID, agentID, userID := uuid.New(), uuid.New(), uuid.New()
	cipher := fakes.Cipher{}
	secret, err := cipher.Encrypt([]byte("secret"), []byte("provider:"+providerID.String()))
	if err != nil {
		t.Fatal(err)
	}
	catalog := fakes.NewCatalogStore()
	provider := domain.Provider{ID: providerID, Name: "Gemini", Kind: domain.ProviderGemini, APIKeyCiphertext: secret, Status: domain.ConnectionOK, CreatedAt: now, UpdatedAt: now}
	if err = catalog.CreateProvider(t.Context(), provider); err != nil {
		t.Fatal(err)
	}
	agent := domain.Agent{ID: agentID, Name: "Trợ lý", ProviderID: providerID, Model: "model", SystemPrompt: "help", ContextWindowTokens: domain.DefaultContextWindowTokens, MaxIterations: 8, TimeoutSeconds: 120, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}
	if configure != nil {
		configure(&agent)
	}
	if err = catalog.CreateAgent(t.Context(), agent); err != nil {
		t.Fatal(err)
	}
	if tools == nil {
		tools = &fakes.ToolSet{}
	}
	store := fakes.NewRunStore()
	llm := &fakes.ScriptedLLM{Steps: steps}
	factory := &fakes.FakeFactory{Client: llm}
	service, err := NewService(Dependencies{Agents: catalog, Providers: catalog, Sessions: store, Messages: store, Snapshots: store, Runs: store, Spans: store, Tx: store, Cipher: cipher, Factory: factory, Tools: fakes.ToolResolver{Set: tools}, Skills: &fakes.SkillResolver{}, Clock: fakes.Clock{Time: now}, IDs: fakes.IDs{}, CancelPoll: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return engineFixture{service: service, store: store, llm: llm, tools: tools, principal: domain.Principal{Kind: domain.PrincipalUser, UserID: userID, Role: domain.RoleMember}, command: inbound.RunCommand{AgentID: agentID, Input: "Xin chào", Mode: domain.RunModeSync, Metadata: json.RawMessage(`{"request":"unit"}`)}, agent: agent}
}

func intRef(value int) *int { return &value }

func TestRunEngineCompletesTextResponseAndAccumulatesUsage(t *testing.T) {
	fixture := newEngineFixture(t, []fakes.LLMStep{{Deltas: []domain.LLMDelta{{Text: "Xin "}, {Text: "chào"}}, Result: domain.LLMResult{Text: "Xin chào", Usage: domain.TokenUsage{InputTokens: intRef(4), OutputTokens: intRef(2)}, FinishReason: "stop"}}}, nil, nil)
	events := []domain.RunEventType{}
	run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
		events = append(events, event.Type)
		return nil
	}))
	if err != nil || run.Status != domain.RunSucceeded || run.Output == nil || *run.Output != "Xin chào" || run.Iterations != 1 || run.Usage.InputTokens == nil || *run.Usage.InputTokens != 4 {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	wantEvents := []domain.RunEventType{domain.EventRunStarted, domain.EventMessageDelta, domain.EventMessageDelta, domain.EventMessageCompleted, domain.EventRunCompleted}
	if !equalEvents(events, wantEvents) {
		t.Fatalf("events=%v", events)
	}
	messages, _ := fixture.store.ListMessages(t.Context(), run.SessionID, 20, nil)
	spans, _ := fixture.store.ListSpans(t.Context(), run.ID, 20, nil)
	if len(messages) != 2 || len(spans) != 2 || messages[1].Role != "assistant" {
		t.Fatalf("messages=%d spans=%d", len(messages), len(spans))
	}
	requests := fixture.llm.Requests()
	if len(requests) != 1 || requests[0].MaxOutputTokens == nil || *requests[0].MaxOutputTokens != domain.DefaultMaxOutputTokens {
		t.Fatalf("provider output cap=%+v", requests)
	}
}

func TestRunEngineUsesResolvedSkillPromptForEveryIteration(t *testing.T) {
	steps := []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "skill-call", Name: "lookup", Arguments: json.RawMessage(`{}`)}}, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Done", FinishReason: "stop"}},
	}
	fixture := newEngineFixture(t, steps, nil, &fakes.ToolSet{Results: []domain.ToolResult{{Content: "result"}}})
	resolver := &fakes.SkillResolver{Prompt: "help\n\n<skill_instructions name=\"Writing\">\nBe concise.\n</skill_instructions>"}
	fixture.service.Skills = resolver
	if _, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command); err != nil {
		t.Fatal(err)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 2 || resolver.Calls != 1 {
		t.Fatalf("requests=%d resolutions=%d", len(requests), resolver.Calls)
	}
	for _, request := range requests {
		if !strings.Contains(request.SystemPrompt, "Be concise.") {
			t.Fatalf("request prompt=%q", request.SystemPrompt)
		}
	}
}

func TestStreamUsesSingleSkillSnapshot(t *testing.T) {
	fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{Text: "Done", FinishReason: "stop"}}}, nil, nil)
	fixture.command.Mode = domain.RunModeStream
	resolutions := 0
	fixture.service.Skills = &fakes.SkillResolver{Resolve: func(_ context.Context, _ uuid.UUID, base string, _ []domain.ToolSpec) (string, error) {
		resolutions++
		if resolutions > 1 {
			return "", errors.New("skill changed after preflight")
		}
		return base + "\n\nSNAPSHOT SKILL", nil
	}}
	run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(context.Context, domain.RunEvent) error { return nil }))
	if err != nil || run.Status != domain.RunSucceeded || resolutions != 1 {
		t.Fatalf("run=%+v err=%v resolutions=%d", run, err, resolutions)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 1 || !strings.Contains(requests[0].SystemPrompt, "SNAPSHOT SKILL") {
		t.Fatalf("requests=%+v", requests)
	}
}

func TestRunEngineExecutesTwoToolRoundsAndReplaysPrivateMetadata(t *testing.T) {
	callMeta := json.RawMessage(`{"call":"opaque"}`)
	assistantMeta := json.RawMessage(`{"parts":[{"text":"opaque"}]}`)
	calls := []domain.ToolCall{{ID: "c1", Name: "lookup", Arguments: json.RawMessage(`{"q":1}`), ProviderMeta: callMeta}, {ID: "c2", Name: "lookup", Arguments: json.RawMessage(`{"q":2}`)}}
	steps := []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: calls[:1], ProviderMeta: assistantMeta, Usage: domain.TokenUsage{InputTokens: intRef(2), OutputTokens: intRef(1)}, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{ToolCalls: calls[1:], Usage: domain.TokenUsage{InputTokens: intRef(3), OutputTokens: intRef(1)}, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Xong", Usage: domain.TokenUsage{InputTokens: intRef(4), OutputTokens: intRef(2)}, FinishReason: "stop"}},
	}
	toolset := &fakes.ToolSet{Results: []domain.ToolResult{{Content: "one", Truncated: true}, {Content: "two"}}}
	fixture := newEngineFixture(t, steps, nil, toolset)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || run.Iterations != 3 || run.Usage.InputTokens == nil || *run.Usage.InputTokens != 9 {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	// The returned run must carry structured tool results, not only the persisted row.
	if len(run.ToolResults) != 2 || run.ToolResults[0].ToolName != "lookup" || string(run.ToolResults[0].Arguments) != `{"q":1}` || string(run.ToolResults[0].Result) != `"one"` || string(run.ToolResults[1].Result) != `"two"` || run.ToolResults[1].IsError {
		t.Fatalf("tool results=%+v", run.ToolResults)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 3 || len(requests[1].Messages) < 3 {
		t.Fatalf("requests=%d", len(requests))
	}
	assistant := requests[1].Messages[1]
	if string(assistant.ProviderMeta) != string(assistantMeta) || len(assistant.ToolCalls) != 1 || string(assistant.ToolCalls[0].ProviderMeta) != string(callMeta) {
		t.Fatal("private provider metadata was not replayed")
	}
	messages, _ := fixture.store.ListMessages(t.Context(), run.SessionID, 20, nil)
	spans, _ := fixture.store.ListSpans(t.Context(), run.ID, 20, nil)
	if len(messages) != 6 || len(spans) != 6 || len(toolset.Calls) != 2 {
		t.Fatalf("messages=%d spans=%d tools=%d", len(messages), len(spans), len(toolset.Calls))
	}
	var sawTruncatedToolSpan bool
	for _, span := range spans {
		if span.Kind != domain.SpanToolCall {
			continue
		}
		var attributes map[string]any
		if json.Unmarshal(span.Attributes, &attributes) == nil && attributes["result_truncated"] == true {
			sawTruncatedToolSpan = true
		}
	}
	if !sawTruncatedToolSpan {
		t.Fatal("missing result_truncated tool span attribute")
	}
}

func TestRunEngineStopsAtIterationAndLoopLimits(t *testing.T) {
	call := func(id, args string) fakes.LLMStep {
		return fakes.LLMStep{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: id, Name: "repeat", Arguments: json.RawMessage(args)}}, FinishReason: "tool_calls"}}
	}
	t.Run("maximum iterations", func(t *testing.T) {
		toolset := &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "repeat", JSONSchema: json.RawMessage(`{"type":"object"}`)}}, Results: []domain.ToolResult{{Content: "1"}, {Content: "2"}}}
		fixture := newEngineFixture(t, []fakes.LLMStep{call("a", `{}`), call("b", `{}`)}, func(agent *domain.Agent) { agent.MaxIterations = 2 }, toolset)
		run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
		if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "max_iterations_reached" || run.Iterations != 2 {
			t.Fatalf("run=%+v err=%v", run, err)
		}
		requests := fixture.llm.Requests()
		if len(toolset.Calls) != 1 || len(requests) != 2 || len(requests[1].Tools) != 0 || requests[1].Messages[len(requests[1].Messages)-1].Text != finalIterationInstruction {
			t.Fatalf("tools executed=%d final request=%+v", len(toolset.Calls), requests[1])
		}
	})
	t.Run("identical loop", func(t *testing.T) {
		fixture := newEngineFixture(t, []fakes.LLMStep{call("a", `{"b":2,"a":1}`), call("b", `{"a":1,"b":2}`), call("c", `{"b":2,"a":1}`)}, nil, &fakes.ToolSet{Results: []domain.ToolResult{{Content: "same"}, {Content: "same"}, {Content: "same"}}})
		run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
		if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "loop_detected" || run.Iterations != 3 {
			t.Fatalf("run=%+v err=%v", run, err)
		}
		requests := fixture.llm.Requests()
		if len(requests) != 3 || !strings.Contains(requests[2].Messages[len(requests[2].Messages)-1].Text, "same result") {
			t.Fatalf("loop warning was not sent before termination: %+v", requests)
		}
	})
}

func TestRunEngineRepairsRepeatedProviderToolCallIDs(t *testing.T) {
	call := func() fakes.LLMStep {
		return fakes.LLMStep{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "reused", Name: "lookup", Arguments: json.RawMessage(`{}`)}}, FinishReason: "tool_calls"}}
	}
	toolset := &fakes.ToolSet{Results: []domain.ToolResult{{Content: "one"}, {Content: "two"}}}
	fixture := newEngineFixture(t, []fakes.LLMStep{call(), call(), {Result: domain.LLMResult{Text: "done", FinishReason: "stop"}}}, nil, toolset)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || len(toolset.Calls) != 2 {
		t.Fatalf("run=%+v calls=%+v err=%v", run, toolset.Calls, err)
	}
	if toolset.Calls[0].ID != "reused" || toolset.Calls[1].ID == "reused" || toolset.Calls[0].ID == toolset.Calls[1].ID || len(toolset.Calls[1].ID) > maxReplayToolCallIDLength {
		t.Fatalf("tool call IDs were not made replay-safe: %+v", toolset.Calls)
	}
}

func TestRunEngineRejectsAnotherActiveRunInConversation(t *testing.T) {
	fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{Text: "must not run", FinishReason: "stop"}}}, nil, nil)
	sessionID := uuid.New()
	userID := fixture.principal.UserID
	now := time.Now()
	if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: sessionID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Conversation", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CreateRun(t.Context(), domain.Run{ID: uuid.New(), AgentID: fixture.agent.ID, SessionID: sessionID, Mode: domain.RunModeSync, Status: domain.RunRunning, Source: domain.RunSourcePlayground, TriggeredByUserID: &userID, Input: domain.RunInput{Message: "first"}, Metadata: json.RawMessage(`{}`), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	fixture.command.SessionID = &sessionID
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if !errors.Is(err, domain.ErrRunInProgress) || run.ID != uuid.Nil || len(fixture.llm.Requests()) != 0 {
		t.Fatalf("run=%+v requests=%d err=%v", run, len(fixture.llm.Requests()), err)
	}
	messages, listErr := fixture.store.ListMessages(t.Context(), sessionID, 20, nil)
	if listErr != nil || len(messages) != 0 {
		t.Fatalf("messages=%+v err=%v", messages, listErr)
	}
}

func TestRunEngineLoopDetectionUsesFullToolResultBeforeTruncation(t *testing.T) {
	call := func(id string) fakes.LLMStep {
		return fakes.LLMStep{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: id, Name: "lookup", Arguments: json.RawMessage(`{}`)}}, FinishReason: "tool_calls"}}
	}
	prefix := string(make([]byte, maxToolResultBytes))
	fixture := newEngineFixture(t, []fakes.LLMStep{call("a"), call("b"), call("c"), {Result: domain.LLMResult{Text: "Xong", FinishReason: "stop"}}}, nil, &fakes.ToolSet{Results: []domain.ToolResult{{Content: prefix + "a"}, {Content: prefix + "b"}, {Content: prefix + "c"}}})
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded {
		t.Fatalf("run=%+v err=%v", run, err)
	}
}

func TestRunEngineWarnsAndStopsWhenDifferentArgumentsReturnSameResult(t *testing.T) {
	steps := make([]fakes.LLMStep, 0, sameResultCriticalThresholdForTest)
	results := make([]domain.ToolResult, 0, sameResultCriticalThresholdForTest)
	for index := 1; index <= sameResultCriticalThresholdForTest; index++ {
		steps = append(steps, fakes.LLMStep{Result: domain.LLMResult{
			ToolCalls:    []domain.ToolCall{{ID: fmt.Sprintf("call-%d", index), Name: "lookup", Arguments: json.RawMessage(fmt.Sprintf(`{"query":%d}`, index))}},
			FinishReason: "tool_calls",
		}})
		results = append(results, domain.ToolResult{Content: "unchanged"})
	}
	tools := &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "lookup", JSONSchema: json.RawMessage(`{"type":"object","required":["query"]}`)}}, Results: results}
	fixture := newEngineFixture(t, steps, nil, tools)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "loop_detected" || run.Iterations != sameResultCriticalThresholdForTest {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	requests := fixture.llm.Requests()
	if len(tools.Calls) != sameResultCriticalThresholdForTest || len(requests) != sameResultCriticalThresholdForTest || !strings.Contains(requests[4].Messages[len(requests[4].Messages)-1].Text, "different arguments") {
		t.Fatalf("calls=%d requests=%+v", len(tools.Calls), requests)
	}
}

const sameResultCriticalThresholdForTest = 6

func TestRunEngineMapsProviderTimeoutAndCancellation(t *testing.T) {
	t.Run("provider authentication", func(t *testing.T) {
		fixture := newEngineFixture(t, []fakes.LLMStep{{Err: domain.ErrProviderAuth}}, nil, nil)
		run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
		if err != nil || run.Failure == nil || run.Failure.Code != "provider_auth_failed" {
			t.Fatalf("run=%+v err=%v", run, err)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		fixture := newEngineFixture(t, nil, nil, nil)
		var terminalContextErr error
		fixture.llm.StreamFunc = func(ctx context.Context, _ domain.LLMRequest, _ func(domain.LLMDelta)) (domain.LLMResult, error) {
			<-ctx.Done()
			return domain.LLMResult{}, ctx.Err()
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		run, err := fixture.service.RunStream(ctx, fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(eventCtx context.Context, event domain.RunEvent) error {
			if event.Type == domain.EventRunFailed {
				terminalContextErr = eventCtx.Err()
			}
			return nil
		}))
		if err != nil || run.Failure == nil || run.Failure.Code != "run_timeout" || terminalContextErr == nil {
			t.Fatalf("run=%+v err=%v", run, err)
		}
	})
	t.Run("agent timeout emits terminal event on request context", func(t *testing.T) {
		fixture := newEngineFixture(t, nil, func(agent *domain.Agent) { agent.TimeoutSeconds = 1 }, nil)
		fixture.llm.StreamFunc = func(ctx context.Context, _ domain.LLMRequest, _ func(domain.LLMDelta)) (domain.LLMResult, error) {
			<-ctx.Done()
			return domain.LLMResult{}, ctx.Err()
		}
		var terminalContextErr error
		run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(eventCtx context.Context, event domain.RunEvent) error {
			if event.Type == domain.EventRunFailed {
				terminalContextErr = eventCtx.Err()
			}
			return nil
		}))
		if err != nil || run.Failure == nil || run.Failure.Code != "run_timeout" || terminalContextErr != nil {
			t.Fatalf("run=%+v terminal context=%v err=%v", run, terminalContextErr, err)
		}
	})
	t.Run("durable cancel request", func(t *testing.T) {
		fixture := newEngineFixture(t, nil, nil, nil)
		started := make(chan struct{})
		fixture.llm.StreamFunc = func(ctx context.Context, _ domain.LLMRequest, _ func(domain.LLMDelta)) (domain.LLMResult, error) {
			close(started)
			<-ctx.Done()
			return domain.LLMResult{}, ctx.Err()
		}
		result := make(chan domain.Run, 1)
		go func() {
			run, _ := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
			result <- run
		}()
		<-started
		runsPage, _ := fixture.store.ListRuns(t.Context(), outbound.RunListOptions{Limit: 1})
		if len(runsPage) != 1 {
			t.Fatal("running execution was not persisted")
		}
		if _, err := fixture.store.RequestRunCancel(t.Context(), runsPage[0].ID, time.Now()); err != nil {
			t.Fatal(err)
		}
		run := <-result
		if run.Status != domain.RunCancelled || run.Failure == nil || run.Failure.Code != "run_cancelled" {
			t.Fatalf("run=%+v", run)
		}
	})
}

func TestRunEngineCancelsWhenEventSinkFails(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	fixture.llm.StreamFunc = func(ctx context.Context, _ domain.LLMRequest, emit func(domain.LLMDelta)) (domain.LLMResult, error) {
		emit(domain.LLMDelta{Text: "partial"})
		<-ctx.Done()
		return domain.LLMResult{}, ctx.Err()
	}
	run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
		if event.Type == domain.EventMessageDelta {
			return errors.New("client gone")
		}
		return nil
	}))
	if err != nil || run.Status != domain.RunCancelled {
		t.Fatalf("run=%+v err=%v", run, err)
	}
}

func TestRunEngineDoesNotExecuteToolAfterEventSinkFailure(t *testing.T) {
	call := domain.ToolCall{ID: "call", Name: "side_effect", Arguments: json.RawMessage(`{}`)}
	tools := &fakes.ToolSet{}
	fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{call}, FinishReason: "tool_calls"}}}, nil, tools)
	run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
		if event.Type == domain.EventToolStarted {
			return errors.New("client gone")
		}
		return nil
	}))
	if err != nil || run.Status != domain.RunCancelled || len(tools.Calls) != 0 {
		t.Fatalf("run=%+v tool calls=%d err=%v", run, len(tools.Calls), err)
	}
}

func equalEvents(got, want []domain.RunEventType) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// Without the status on the span, the activity timeline could only say a tool failed,
// so diagnosing a failure meant reading the upstream service's own logs.
func TestRunEngineRecordsToolHTTPStatusOnSpan(t *testing.T) {
	serverError := 500
	steps := []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "c1", Name: "lookup", Arguments: json.RawMessage(`{"q":1}`)}}, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Không tra cứu được", FinishReason: "stop"}},
	}
	toolset := &fakes.ToolSet{Results: []domain.ToolResult{{Content: "Internal Server Error", IsError: true, StatusCode: &serverError}}}
	fixture := newEngineFixture(t, steps, nil, toolset)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	spans, _ := fixture.store.ListSpans(t.Context(), run.ID, 20, nil)
	var checked bool
	for _, span := range spans {
		if span.Kind != domain.SpanToolCall {
			continue
		}
		checked = true
		var attributes map[string]any
		if json.Unmarshal(span.Attributes, &attributes) != nil || attributes["status_code"] != float64(serverError) {
			t.Fatalf("span thiếu status_code: %s", span.Attributes)
		}
		if span.Status != domain.SpanError || span.ErrorMessage == nil || !strings.Contains(*span.ErrorMessage, "500") {
			t.Fatalf("span không nêu mã lỗi: %+v", span)
		}
		// The timeline is read by people, not by the model.
		if strings.Contains(*span.ErrorMessage, "tham số") {
			t.Fatalf("chỉ dẫn dành cho mô hình lọt vào span: %q", *span.ErrorMessage)
		}
	}
	if !checked {
		t.Fatal("không có span tool nào")
	}
}

// The boundary is worthless unless it actually reaches the provider on every run mode,
// and unless it survives alongside the operator's prompt rather than replacing it.
func TestRunEngineStatesCapabilityBoundaryInSystemPrompt(t *testing.T) {
	toolset := &fakes.ToolSet{Tools: []domain.ToolSpec{
		{Name: "http_precheck_transfer", Description: "Chấm điểm rủi ro một lệnh chuyển tiền."},
		{Name: "http_take_action", Description: "Chốt hành động cuối."},
	}}
	steps := []fakes.LLMStep{{Result: domain.LLMResult{Text: "Xong", FinishReason: "stop"}}}
	fixture := newEngineFixture(t, steps, nil, toolset)
	fixture.service.Skills = &fakes.SkillResolver{Prompt: "Bạn là trợ lý chống lừa đảo chuyển tiền."}
	if _, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command); err != nil {
		t.Fatal(err)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	prompt := requests[0].SystemPrompt
	if !strings.Contains(prompt, "Bạn là trợ lý chống lừa đảo chuyển tiền.") {
		t.Fatalf("chỉ dẫn của người dựng bị thay mất: %q", prompt)
	}
	for _, name := range []string{"http_precheck_transfer", "http_take_action"} {
		if !strings.Contains(prompt, name) {
			t.Fatalf("thiếu %q trong mục năng lực: %q", name, prompt)
		}
	}
	if !strings.Contains(prompt, "Never promise") || !strings.Contains(prompt, "never mention tool names") {
		t.Fatalf("thiếu luật ranh giới năng lực: %q", prompt)
	}
}

// Stating the time is pointless unless it reaches the provider, and the capability
// boundary has to stay last so it still reads as the final word.
func TestRunEngineStatesCurrentTimeBeforeCapabilityBoundary(t *testing.T) {
	toolset := &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "http_get_insights", Description: "Đọc insight chi tiêu."}}}
	steps := []fakes.LLMStep{{Result: domain.LLMResult{Text: "Xong", FinishReason: "stop"}}}
	fixture := newEngineFixture(t, steps, nil, toolset)
	if _, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command); err != nil {
		t.Fatal(err)
	}
	prompt := fixture.llm.Requests()[0].SystemPrompt
	if !strings.Contains(prompt, "## Current date and time") {
		t.Fatalf("thiếu mục thời gian: %q", prompt)
	}
	if !strings.Contains(prompt, "Never assume a year") {
		t.Fatalf("thiếu luật cấm tự đoán năm: %q", prompt)
	}
	if strings.Index(prompt, "## Current date and time") > strings.Index(prompt, "## Capabilities") {
		t.Fatal("mục thời gian phải đứng trước mục năng lực")
	}
}

// A skill names the tools it needs, and the name it writes is the operator-facing slug
// rather than the name generated for the provider. The run is the only place that knows
// both, so it has to hand the resolver the tools it resolved for this run.
func TestRunEngineGivesSkillResolverTheResolvedTools(t *testing.T) {
	toolset := &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "http_get_insights", Ref: "get_insights", Description: "Đọc insight chi tiêu."}}}
	steps := []fakes.LLMStep{{Result: domain.LLMResult{Text: "Xong", FinishReason: "stop"}}}
	fixture := newEngineFixture(t, steps, nil, toolset)
	resolver := &fakes.SkillResolver{Resolve: func(_ context.Context, _ uuid.UUID, base string, specs []domain.ToolSpec) (string, error) {
		return base + "\n\nĐọc " + domain.RewriteSkillToolRefs("$get_insights", specs) + ".", nil
	}}
	fixture.service.Skills = resolver
	if _, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command); err != nil {
		t.Fatal(err)
	}
	if len(resolver.Specs) != 1 || resolver.Specs[0].Ref != "get_insights" {
		t.Fatalf("resolver không nhận được tool của run: %+v", resolver.Specs)
	}
	if prompt := fixture.llm.Requests()[0].SystemPrompt; !strings.Contains(prompt, "Đọc http_get_insights.") {
		t.Fatalf("tham chiếu chưa đổi thành tên runtime: %q", prompt)
	}
}

// The async worker resolves the prompt and the tool set in one setup block, so the order
// of those two steps decides whether a skill's tool references can be rewritten at all.
func TestAsyncRunGivesSkillResolverTheResolvedTools(t *testing.T) {
	toolset := &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "http_get_insights", Ref: "get_insights", Description: "Đọc insight chi tiêu."}}}
	steps := []fakes.LLMStep{{Result: domain.LLMResult{Text: "Xong", FinishReason: "stop"}}}
	fixture := newEngineFixture(t, steps, nil, toolset)
	resolver := &fakes.SkillResolver{Resolve: func(_ context.Context, _ uuid.UUID, base string, specs []domain.ToolSpec) (string, error) {
		return base + "\n\nĐọc " + domain.RewriteSkillToolRefs("$get_insights", specs) + ".", nil
	}}
	fixture.service.Skills = resolver
	_, job := seedQueuedRun(t, fixture)
	if err := fixture.service.ProcessJob(t.Context(), "worker-a", job); err != nil {
		t.Fatal(err)
	}
	if len(resolver.Specs) != 1 || resolver.Specs[0].Ref != "get_insights" {
		t.Fatalf("resolver không nhận được tool của run: %+v", resolver.Specs)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 1 || !strings.Contains(requests[0].SystemPrompt, "Đọc http_get_insights.") {
		t.Fatalf("tham chiếu chưa đổi thành tên runtime: %+v", requests)
	}
}
