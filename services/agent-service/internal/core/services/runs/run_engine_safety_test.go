package runs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

type panicAgentRepository struct{ outbound.AgentRepository }

func (panicAgentRepository) GetAgent(context.Context, uuid.UUID) (domain.Agent, error) {
	panic("agent repository panic")
}

type panicOnWatcherStopRepository struct {
	outbound.RunRepository
	entered chan struct{}
	once    sync.Once
}

func (r *panicOnWatcherStopRepository) RunCancelRequested(ctx context.Context, _ uuid.UUID) (bool, error) {
	r.once.Do(func() { close(r.entered) })
	<-ctx.Done()
	panic("watcher panic must not leak")
}

type cancelAwareWatcherRepository struct {
	outbound.RunRepository
	entered chan struct{}
	once    sync.Once
}

func (r *cancelAwareWatcherRepository) RunCancelRequested(ctx context.Context, _ uuid.UUID) (bool, error) {
	r.once.Do(func() { close(r.entered) })
	<-ctx.Done()
	return false, ctx.Err()
}

type panicOnceGetRunRepository struct {
	outbound.RunRepository
	once sync.Once
}

func (r *panicOnceGetRunRepository) GetRun(ctx context.Context, id uuid.UUID) (domain.Run, error) {
	panicked := false
	r.once.Do(func() { panicked = true })
	if panicked {
		panic("claim lookup panic must not leak")
	}
	return r.RunRepository.GetRun(ctx, id)
}

type panicAfterStartRepository struct {
	outbound.RunRepository
	once sync.Once
}

func (r *panicAfterStartRepository) StartQueuedRun(ctx context.Context, id uuid.UUID, at time.Time) (domain.Run, error) {
	run, err := r.RunRepository.StartQueuedRun(ctx, id, at)
	panicked := false
	r.once.Do(func() { panicked = true })
	if err == nil && panicked {
		panic("post-claim panic must not leak")
	}
	return run, err
}

func TestRunEngineRecoversPanicsAndTerminalizesRun(t *testing.T) {
	t.Run("LLM panic", func(t *testing.T) {
		fixture := newEngineFixture(t, nil, nil, nil)
		var logs bytes.Buffer
		fixture.service.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
		fixture.llm.StreamFunc = func(context.Context, domain.LLMRequest, func(domain.LLMDelta)) (domain.LLMResult, error) {
			panic("credential=must-not-leak")
		}

		run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
		if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "internal" {
			t.Fatalf("run=%+v err=%v", run, err)
		}
		persisted, getErr := fixture.store.GetRun(t.Context(), run.ID)
		spans, spanErr := fixture.store.ListSpans(t.Context(), run.ID, 20, nil)
		if getErr != nil || spanErr != nil || !persisted.Terminal() || len(spans) != 1 || spans[0].Kind != domain.SpanRun {
			t.Fatalf("persisted=%+v spans=%+v getErr=%v spanErr=%v", persisted, spans, getErr, spanErr)
		}
		if strings.Contains(run.Failure.Message, "must-not-leak") || strings.Contains(logs.String(), "must-not-leak") || !strings.Contains(logs.String(), run.ID.String()) {
			t.Fatalf("unsafe panic diagnostic: failure=%q logs=%q", run.Failure.Message, logs.String())
		}
	})

	t.Run("tool panic", func(t *testing.T) {
		call := domain.ToolCall{ID: "panic-call", Name: "side_effect", Arguments: json.RawMessage(`{}`)}
		tools := &fakes.ToolSet{Run: func(context.Context, domain.ToolCall) domain.ToolResult {
			panic("tool result must not leak")
		}}
		fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{call}, FinishReason: "tool_calls"}}}, nil, tools)

		run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
		if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "internal" || len(tools.Calls) != 1 {
			t.Fatalf("run=%+v calls=%d err=%v", run, len(tools.Calls), err)
		}
		messages, messageErr := fixture.store.ListMessages(t.Context(), run.SessionID, 20, nil)
		spans, spanErr := fixture.store.ListSpans(t.Context(), run.ID, 20, nil)
		if messageErr != nil || spanErr != nil || len(messages) != 2 || len(messages[1].ToolCalls) != 1 || messages[1].ToolCalls[0].ID != call.ID {
			t.Fatalf("messages=%+v spans=%+v messageErr=%v spanErr=%v", messages, spans, messageErr, spanErr)
		}
		for _, message := range messages {
			if message.Role == "tool" {
				t.Fatalf("panic created a synthetic durable tool result: %+v", messages)
			}
		}
		if len(spans) != 2 || countRootSpans(spans) != 1 {
			t.Fatalf("spans=%+v", spans)
		}
	})

	t.Run("event sink panic before tool side effect", func(t *testing.T) {
		call := domain.ToolCall{ID: "sink-panic-call", Name: "side_effect", Arguments: json.RawMessage(`{}`)}
		tools := &fakes.ToolSet{}
		fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{call}, FinishReason: "tool_calls"}}}, nil, tools)
		run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
			if event.Type == domain.EventToolStarted {
				panic("event sink secret")
			}
			return nil
		}))
		if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "internal" || len(tools.Calls) != 0 {
			t.Fatalf("run=%+v calls=%d err=%v", run, len(tools.Calls), err)
		}
	})

	t.Run("terminal event sink panic keeps persisted success", func(t *testing.T) {
		fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{Text: "done", FinishReason: "stop"}}}, nil, nil)
		run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
			if event.Type == domain.EventRunCompleted {
				panic("terminal callback panic")
			}
			return nil
		}))
		persisted, getErr := fixture.store.GetRun(t.Context(), run.ID)
		if err != nil || getErr != nil || run.Status != domain.RunSucceeded || persisted.Status != domain.RunSucceeded {
			t.Fatalf("run=%+v persisted=%+v err=%v getErr=%v", run, persisted, err, getErr)
		}
	})

	t.Run("cancel watcher panic is joined before success", func(t *testing.T) {
		fixture := newEngineFixture(t, nil, nil, nil)
		repository := &panicOnWatcherStopRepository{RunRepository: fixture.service.Runs, entered: make(chan struct{})}
		fixture.service.Runs = repository
		fixture.llm.StreamFunc = func(context.Context, domain.LLMRequest, func(domain.LLMDelta)) (domain.LLMResult, error) {
			<-repository.entered
			return domain.LLMResult{Text: "done", FinishReason: "stop"}, nil
		}

		run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
		if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "internal" {
			t.Fatalf("run=%+v err=%v", run, err)
		}
	})

	t.Run("stopping cancel watcher preserves successful run", func(t *testing.T) {
		fixture := newEngineFixture(t, nil, nil, nil)
		repository := &cancelAwareWatcherRepository{RunRepository: fixture.service.Runs, entered: make(chan struct{})}
		fixture.service.Runs = repository
		fixture.llm.StreamFunc = func(context.Context, domain.LLMRequest, func(domain.LLMDelta)) (domain.LLMResult, error) {
			<-repository.entered
			return domain.LLMResult{Text: "done", FinishReason: "stop"}, nil
		}

		run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
		if err != nil || run.Status != domain.RunSucceeded || run.Failure != nil {
			t.Fatalf("run=%+v err=%v", run, err)
		}
	})
}

func TestAsyncRunPanicIsTerminalAndNotReplayed(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	fixture.llm.StreamFunc = func(context.Context, domain.LLMRequest, func(domain.LLMDelta)) (domain.LLMResult, error) {
		panic("async provider panic")
	}
	runID, job := seedQueuedRun(t, fixture)

	if err := fixture.service.ProcessJob(t.Context(), "worker-a", job); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.ProcessJob(t.Context(), "worker-b", job); err != nil {
		t.Fatal(err)
	}
	persisted, err := fixture.store.GetRun(t.Context(), runID)
	spans, spanErr := fixture.store.ListSpans(t.Context(), runID, 20, nil)
	if err != nil || spanErr != nil || persisted.Status != domain.RunFailed || persisted.Failure == nil || persisted.Failure.Code != "internal" {
		t.Fatalf("persisted=%+v spans=%+v err=%v spanErr=%v", persisted, spans, err, spanErr)
	}
	if requests := fixture.llm.Requests(); len(requests) != 1 || len(spans) != 1 || spans[0].Kind != domain.SpanRun {
		t.Fatalf("requests=%d spans=%+v", len(requests), spans)
	}
}

func TestAsyncToolPanicIsTerminalAndSideEffectIsNotReplayed(t *testing.T) {
	call := domain.ToolCall{ID: "async-side-effect", Name: "side_effect", Arguments: json.RawMessage(`{"value":1}`)}
	tools := &fakes.ToolSet{Run: func(context.Context, domain.ToolCall) domain.ToolResult {
		panic("async tool panic must not leak")
	}}
	fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{call}, FinishReason: "tool_calls"}}}, nil, tools)
	runID, job := seedQueuedRun(t, fixture)

	if err := fixture.service.ProcessJob(t.Context(), "worker-a", job); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.ProcessJob(t.Context(), "worker-b", job); err != nil {
		t.Fatal(err)
	}
	persisted, err := fixture.store.GetRun(t.Context(), runID)
	messages, messageErr := fixture.store.ListMessages(t.Context(), persisted.SessionID, 20, nil)
	spans, spanErr := fixture.store.ListSpans(t.Context(), runID, 20, nil)
	if err != nil || messageErr != nil || spanErr != nil || persisted.Status != domain.RunFailed || persisted.Failure == nil || persisted.Failure.Code != "internal" {
		t.Fatalf("persisted=%+v messages=%+v spans=%+v err=%v messageErr=%v spanErr=%v", persisted, messages, spans, err, messageErr, spanErr)
	}
	if len(tools.Calls) != 1 || len(fixture.llm.Requests()) != 1 || len(messages) != 1 || len(messages[0].ToolCalls) != 1 || messages[0].ToolCalls[0].ID != call.ID || countRootSpans(spans) != 1 {
		t.Fatalf("toolCalls=%d requests=%d messages=%+v spans=%+v", len(tools.Calls), len(fixture.llm.Requests()), messages, spans)
	}
	for _, message := range messages {
		if message.Role == "tool" {
			t.Fatalf("panic created a synthetic durable tool result: %+v", messages)
		}
	}
}

func TestAsyncClaimPanicsAreContainedWithoutUnsafeReplay(t *testing.T) {
	t.Run("panic before ownership leaves queued run retryable", func(t *testing.T) {
		fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{Text: "done", FinishReason: "stop"}}}, nil, nil)
		runID, job := seedQueuedRun(t, fixture)
		fixture.service.Runs = &panicOnceGetRunRepository{RunRepository: fixture.service.Runs}

		if err := fixture.service.ProcessJob(t.Context(), "worker-a", job); !errors.Is(err, errExecutionPanic) {
			t.Fatalf("first claim err=%v", err)
		}
		queued, err := fixture.store.GetRun(t.Context(), runID)
		if err != nil || queued.Status != domain.RunQueued || len(fixture.llm.Requests()) != 0 {
			t.Fatalf("queued=%+v requests=%d err=%v", queued, len(fixture.llm.Requests()), err)
		}
		if err = fixture.service.ProcessJob(t.Context(), "worker-b", job); err != nil {
			t.Fatal(err)
		}
		persisted, err := fixture.store.GetRun(t.Context(), runID)
		if err != nil || persisted.Status != domain.RunSucceeded || len(fixture.llm.Requests()) != 1 {
			t.Fatalf("persisted=%+v requests=%d err=%v", persisted, len(fixture.llm.Requests()), err)
		}
	})

	t.Run("panic after ownership terminalizes without execution", func(t *testing.T) {
		fixture := newEngineFixture(t, []fakes.LLMStep{{Result: domain.LLMResult{Text: "must not run", FinishReason: "stop"}}}, nil, nil)
		runID, job := seedQueuedRun(t, fixture)
		fixture.service.Runs = &panicAfterStartRepository{RunRepository: fixture.service.Runs}

		if err := fixture.service.ProcessJob(t.Context(), "worker-a", job); err != nil {
			t.Fatal(err)
		}
		persisted, err := fixture.store.GetRun(t.Context(), runID)
		spans, spanErr := fixture.store.ListSpans(t.Context(), runID, 20, nil)
		if err != nil || spanErr != nil || persisted.Status != domain.RunFailed || persisted.Failure == nil || persisted.Failure.Code != "internal" || len(fixture.llm.Requests()) != 0 || countRootSpans(spans) != 1 {
			t.Fatalf("persisted=%+v requests=%d spans=%+v err=%v spanErr=%v", persisted, len(fixture.llm.Requests()), spans, err, spanErr)
		}
	})
}

func TestAsyncSetupPanicIsTerminalized(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	var logs bytes.Buffer
	fixture.service.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	fixture.service.Agents = panicAgentRepository{AgentRepository: fixture.service.Agents}
	runID, job := seedQueuedRun(t, fixture)

	if err := fixture.service.ProcessJob(t.Context(), "worker-a", job); err != nil {
		t.Fatal(err)
	}
	persisted, err := fixture.store.GetRun(t.Context(), runID)
	spans, spanErr := fixture.store.ListSpans(t.Context(), runID, 20, nil)
	if err != nil || spanErr != nil || persisted.Status != domain.RunFailed || persisted.Failure == nil || persisted.Failure.Code != "internal" {
		t.Fatalf("persisted=%+v spans=%+v err=%v spanErr=%v", persisted, spans, err, spanErr)
	}
	if len(fixture.llm.Requests()) != 0 || len(spans) != 1 || countRootSpans(spans) != 1 || strings.Contains(logs.String(), "agent repository panic") {
		t.Fatalf("requests=%d spans=%+v logs=%q", len(fixture.llm.Requests()), spans, logs.String())
	}
}

func seedQueuedRun(t *testing.T, fixture engineFixture) (uuid.UUID, domain.RunJob) {
	t.Helper()
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	sessionID, runID := uuid.New(), uuid.New()
	userID := fixture.principal.UserID
	if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: sessionID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Async", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	queuedAt := now
	queued := domain.Run{ID: runID, AgentID: fixture.agent.ID, SessionID: sessionID, Mode: domain.RunModeAsync, Status: domain.RunQueued, Source: domain.RunSourcePlayground, TriggeredByUserID: &userID, Input: domain.RunInput{Message: "run once"}, Metadata: json.RawMessage(`{}`), QueuedAt: &queuedAt, CreatedAt: now}
	if err := fixture.store.CreateRun(t.Context(), queued); err != nil {
		t.Fatal(err)
	}
	return runID, domain.RunJob{RunID: runID, AvailableAt: now}
}

func countRootSpans(spans []domain.Span) int {
	count := 0
	for _, span := range spans {
		if span.Kind == domain.SpanRun {
			count++
		}
	}
	return count
}
