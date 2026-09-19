package runs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

func TestRunCompactsLongHistoryPersistsCheckpointAndReusesIt(t *testing.T) {
	steps := []fakes.LLMStep{
		{Result: domain.LLMResult{Text: "Durable summary", Usage: domain.TokenUsage{InputTokens: intRef(100), OutputTokens: intRef(20)}, FinishReason: "stop"}},
		{Result: domain.LLMResult{Text: "First answer", Usage: domain.TokenUsage{InputTokens: intRef(40), OutputTokens: intRef(5)}, FinishReason: "stop"}},
		{Result: domain.LLMResult{Text: "Second answer", Usage: domain.TokenUsage{InputTokens: intRef(30), OutputTokens: intRef(4)}, FinishReason: "stop"}},
	}
	fixture := newEngineFixture(t, steps, func(agent *domain.Agent) {
		agent.ContextWindowTokens = domain.MinContextWindowTokens
		agent.MaxOutputTokens = intRef(1024)
	}, nil)
	sessionID := seedLongConversation(t, fixture, 12, 1200)
	firstCommand := fixture.command
	firstCommand.SessionID = &sessionID
	first, err := fixture.service.RunSync(t.Context(), fixture.principal, firstCommand)
	if err != nil || first.Status != domain.RunSucceeded || first.Usage.InputTokens == nil || *first.Usage.InputTokens != 140 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	snapshot, err := fixture.store.GetLatestContextSnapshot(t.Context(), sessionID)
	if err != nil || snapshot.Version != 1 || snapshot.CoveredThroughSeq < 1 || snapshot.Summary != "Durable summary" {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	messages, _ := fixture.store.ListMessagesAfterSeq(t.Context(), sessionID, 0, 100)
	if len(messages) != 14 {
		t.Fatalf("raw transcript changed: %d messages", len(messages))
	}
	spans, _ := fixture.store.ListSpans(t.Context(), first.ID, 20, nil)
	names := map[string]bool{}
	for _, span := range spans {
		names[span.Name] = true
	}
	if len(spans) != 3 || !names["llm.compact_context"] || !names["llm.generate"] || !names["run.execute"] {
		t.Fatalf("spans=%+v", spans)
	}
	session, _ := fixture.store.GetSession(t.Context(), sessionID)
	if session.LastPromptEstimatedTokens == nil || session.LastPromptTokens == nil || *session.LastPromptTokens != 40 {
		t.Fatalf("calibration=%+v", session)
	}
	secondCommand := fixture.command
	secondCommand.Input = "Continue"
	secondCommand.SessionID = &sessionID
	second, err := fixture.service.RunSync(t.Context(), fixture.principal, secondCommand)
	if err != nil || second.Status != domain.RunSucceeded {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 3 || !strings.Contains(requests[2].Messages[0].Text, "Durable summary") {
		t.Fatalf("checkpoint was not reused: requests=%d messages=%+v", len(requests), requests[2].Messages)
	}
}

func TestRunFailsClosedBeforeOversizedMainRequest(t *testing.T) {
	fixture := newEngineFixture(t, nil, func(agent *domain.Agent) {
		agent.ContextWindowTokens = domain.MinContextWindowTokens
		agent.MaxOutputTokens = intRef(1024)
	}, nil)
	fixture.command.Input = strings.Repeat("x", 20_000)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunFailed || run.Failure == nil || run.Failure.Code != "context_limit_exceeded" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if len(fixture.llm.Requests()) != 0 {
		t.Fatal("oversized request reached the provider")
	}
}

func TestCompactionFailureFallsBackWhileUnderHardLimit(t *testing.T) {
	fixture := newEngineFixture(t, []fakes.LLMStep{
		{Err: domain.ErrProviderUnreachable},
		{Result: domain.LLMResult{Text: "Fallback answer", FinishReason: "stop"}},
	}, func(agent *domain.Agent) {
		agent.ContextWindowTokens = domain.MinContextWindowTokens
		agent.MaxOutputTokens = intRef(1024)
	}, nil)
	sessionID := seedLongConversation(t, fixture, 12, 1200)
	fixture.command.SessionID = &sessionID
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || run.Output == nil || *run.Output != "Fallback answer" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if _, err = fixture.store.GetLatestContextSnapshot(t.Context(), sessionID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("failed summary created checkpoint: %v", err)
	}
}

func TestTruncatedCompactionRetriesAndKeepsTheConversationAnswerable(t *testing.T) {
	fixture := newEngineFixture(t, []fakes.LLMStep{
		{Result: domain.LLMResult{Text: "Incomplete summary", Usage: domain.TokenUsage{InputTokens: intRef(100), OutputTokens: intRef(20)}, FinishReason: "max_tokens"}},
		{Result: domain.LLMResult{Text: "Durable summary", Usage: domain.TokenUsage{InputTokens: intRef(60), OutputTokens: intRef(12)}, FinishReason: "stop"}},
		{Result: domain.LLMResult{Text: "First answer", Usage: domain.TokenUsage{InputTokens: intRef(40), OutputTokens: intRef(5)}, FinishReason: "stop"}},
	}, func(agent *domain.Agent) {
		agent.ContextWindowTokens = domain.MinContextWindowTokens
		agent.MaxOutputTokens = intRef(1024)
	}, nil)
	sessionID := seedLongConversation(t, fixture, 12, 1200)
	fixture.command.SessionID = &sessionID
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || run.Output == nil || *run.Output != "First answer" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	snapshot, err := fixture.store.GetLatestContextSnapshot(t.Context(), sessionID)
	if err != nil || snapshot.Summary != "Durable summary" {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	// Mốc thời gian phải là lúc bản tóm tắt được nhận, không phải giá trị rỗng: một
	// checkpoint mang năm 0001 vẫn ghi được vào cột NOT NULL và chỉ lộ ra khi đọc lại.
	if snapshot.CreatedAt.IsZero() {
		t.Fatalf("checkpoint không có mốc thời gian: %+v", snapshot)
	}
	// Both attempts stay visible: the truncated one is what explains the extra cost.
	spans, _ := fixture.store.ListSpans(t.Context(), run.ID, 20, nil)
	attempts := 0
	for _, span := range spans {
		if span.Name == "llm.compact_context" {
			attempts++
		}
	}
	if attempts != 2 {
		t.Fatalf("compaction attempts=%d spans=%+v", attempts, spans)
	}
	if run.Usage.InputTokens == nil || *run.Usage.InputTokens != 200 || run.Usage.OutputTokens == nil || *run.Usage.OutputTokens != 37 {
		t.Fatalf("usage=%+v", run.Usage)
	}
}

func TestCompactionTruncatedEveryAttemptDoesNotAdvanceCheckpointAndCountsUsage(t *testing.T) {
	truncated := fakes.LLMStep{Result: domain.LLMResult{Text: "Incomplete summary", Usage: domain.TokenUsage{InputTokens: intRef(100), OutputTokens: intRef(20)}, FinishReason: "max_tokens"}}
	fixture := newEngineFixture(t, []fakes.LLMStep{
		truncated, truncated, truncated,
		{Result: domain.LLMResult{Text: "Fallback answer", Usage: domain.TokenUsage{InputTokens: intRef(30), OutputTokens: intRef(4)}, FinishReason: "stop"}},
	}, func(agent *domain.Agent) {
		// A window wide enough for retries to have somewhere to go: each attempt
		// summarises half as much and is allowed more room to write.
		agent.ContextWindowTokens = domain.DefaultContextWindowTokens
		agent.MaxOutputTokens = intRef(1024)
	}, nil)
	sessionID := seedLongConversation(t, fixture, 24, 3000)
	fixture.command.SessionID = &sessionID
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || run.Output == nil || *run.Output != "Fallback answer" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	spans, _ := fixture.store.ListSpans(t.Context(), run.ID, 20, nil)
	attempts := 0
	for _, span := range spans {
		if span.Name == "llm.compact_context" {
			attempts++
		}
	}
	if attempts != summaryAttempts {
		t.Fatalf("compaction attempts=%d, want %d", attempts, summaryAttempts)
	}
	if run.Usage.InputTokens == nil || *run.Usage.InputTokens != 330 || run.Usage.OutputTokens == nil || *run.Usage.OutputTokens != 64 {
		t.Fatalf("usage=%+v", run.Usage)
	}
	if _, err = fixture.store.GetLatestContextSnapshot(t.Context(), sessionID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("truncated summary advanced checkpoint: %v", err)
	}
}

func TestToolLoopCompactionPreservesCurrentUserRequest(t *testing.T) {
	tools := &fakes.ToolSet{
		Tools:   []domain.ToolSpec{{Name: "lookup", JSONSchema: json.RawMessage(`{"type":"object"}`)}},
		Results: []domain.ToolResult{{Content: strings.Repeat("tool-result-", 1200)}},
	}
	fixture := newEngineFixture(t, nil, func(agent *domain.Agent) {
		agent.ContextWindowTokens = domain.MinContextWindowTokens
		agent.MaxOutputTokens = intRef(1024)
	}, tools)
	sessionID := seedLongConversation(t, fixture, 16, 900)
	fixture.command.SessionID = &sessionID
	fixture.command.Input = "CURRENT USER REQUEST MUST SURVIVE"
	mainCalls := 0
	fixture.llm.StreamFunc = func(_ context.Context, request domain.LLMRequest, _ func(domain.LLMDelta)) (domain.LLMResult, error) {
		if request.SystemPrompt == summarySystemPrompt {
			return domain.LLMResult{Text: "Earlier durable facts", FinishReason: "stop"}, nil
		}
		mainCalls++
		if mainCalls == 1 {
			return domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "lookup-1", Name: "lookup", Arguments: json.RawMessage(`{}`)}}, FinishReason: "tool_calls"}, nil
		}
		return domain.LLMResult{Text: "Done", FinishReason: "stop"}, nil
	}
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || mainCalls != 2 {
		t.Fatalf("run=%+v main calls=%d err=%v", run, mainCalls, err)
	}
	for _, request := range fixture.llm.Requests() {
		if request.SystemPrompt == summarySystemPrompt {
			if strings.Contains(request.Messages[0].Text, fixture.command.Input) {
				t.Fatal("current request was included in a compaction payload")
			}
			continue
		}
		found := false
		for _, message := range request.Messages {
			found = found || strings.Contains(message.Text, fixture.command.Input)
		}
		if !found {
			t.Fatalf("current request missing from main request: %+v", request.Messages)
		}
	}
}

type winningSnapshotRepository struct {
	store            *fakes.RunStore
	coveredThrough   int64
	injectedConflict bool
}

func (r *winningSnapshotRepository) GetLatestContextSnapshot(ctx context.Context, sessionID uuid.UUID) (domain.ContextSnapshot, error) {
	return r.store.GetLatestContextSnapshot(ctx, sessionID)
}

func (r *winningSnapshotRepository) CreateContextSnapshotCAS(ctx context.Context, value domain.ContextSnapshot, expectedVersion int64) (bool, error) {
	if !r.injectedConflict {
		r.injectedConflict = true
		winner := value
		winner.ID = uuid.New()
		winner.CoveredThroughSeq = r.coveredThrough
		winner.Summary = "Winning concurrent summary"
		if created, err := r.store.CreateContextSnapshotCAS(ctx, winner, expectedVersion); err != nil || !created {
			return false, err
		}
		return false, nil
	}
	return r.store.CreateContextSnapshotCAS(ctx, value, expectedVersion)
}

type recordingMessageRepository struct {
	outbound.MessageRepository
	afterSeqs []int64
}

func (r *recordingMessageRepository) ListMessagesAfterSeq(ctx context.Context, sessionID uuid.UUID, afterSeq int64, limit int) ([]domain.Message, error) {
	r.afterSeqs = append(r.afterSeqs, afterSeq)
	return r.MessageRepository.ListMessagesAfterSeq(ctx, sessionID, afterSeq, limit)
}

func TestCheckpointConflictSkipsMessagesCoveredByWinner(t *testing.T) {
	fixture := newEngineFixture(t, []fakes.LLMStep{
		{Result: domain.LLMResult{Text: "Losing summary", FinishReason: "stop"}},
		{Result: domain.LLMResult{Text: "Done", FinishReason: "stop"}},
	}, func(agent *domain.Agent) {
		agent.ContextWindowTokens = domain.MinContextWindowTokens
		agent.MaxOutputTokens = intRef(1024)
	}, nil)
	sessionID := seedLongConversation(t, fixture, 40, 1000)
	fixture.command.SessionID = &sessionID
	fixture.service.ContextPageSize = 26
	recording := &recordingMessageRepository{MessageRepository: fixture.store}
	fixture.service.Messages = recording
	fixture.service.Snapshots = &winningSnapshotRepository{store: fixture.store, coveredThrough: 35}
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if len(recording.afterSeqs) < 2 || recording.afterSeqs[0] != 0 || recording.afterSeqs[1] != 35 {
		t.Fatalf("pagination did not adopt winner coverage: %v", recording.afterSeqs)
	}
}

func TestContextPageCutDoesNotSplitToolExchange(t *testing.T) {
	messages := []domain.Message{
		{Role: "user"},
		{Role: "assistant", ToolCalls: []domain.ToolCall{{ID: "a", Name: "lookup"}}},
		{Role: "tool"},
		{Role: "tool"},
	}
	if cut := safeMessagePageCut(messages, 3); cut != 1 {
		t.Fatalf("cut=%d", cut)
	}
}

func seedLongConversation(t *testing.T, fixture engineFixture, count, size int) uuid.UUID {
	t.Helper()
	sessionID := uuid.New()
	now := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	userID := fixture.principal.UserID
	if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: sessionID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Long conversation", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < count; index++ {
		if _, err := fixture.store.AppendMessage(t.Context(), domain.Message{ID: uuid.New(), SessionID: sessionID, Role: "user", Content: strings.Repeat(string(rune('a'+index%20)), size), CreatedAt: now.Add(time.Duration(index) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	return sessionID
}

var _ inbound.RunUseCase = (*Service)(nil)
