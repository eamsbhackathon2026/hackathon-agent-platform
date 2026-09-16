package runs

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

func TestRunEngineRetriesEmptyRepliesWithoutPersistingTransientContext(t *testing.T) {
	meta := json.RawMessage(`{"opaque":true}`)
	fixture := newEngineFixture(t, []fakes.LLMStep{
		{Result: domain.LLMResult{FinishReason: "stop", ProviderMeta: meta}},
		{Result: domain.LLMResult{Text: " \n ", FinishReason: "stop"}},
		{Result: domain.LLMResult{Text: "Visible answer", FinishReason: "stop"}},
	}, nil, nil)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || run.Iterations != 3 || run.Output == nil || *run.Output != "Visible answer" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 3 || requests[1].Messages[len(requests[1].Messages)-1].Text != emptyReplyInstruction || requests[2].Messages[len(requests[2].Messages)-1].Text != emptyReplyInstruction {
		t.Fatalf("requests=%+v", requests)
	}
	if len(requests[1].Messages) < 3 || string(requests[1].Messages[len(requests[1].Messages)-2].ProviderMeta) != string(meta) {
		t.Fatalf("first retry lost opaque response context: %+v", requests[1].Messages)
	}
	messages, listErr := fixture.store.ListMessages(t.Context(), run.SessionID, 20, nil)
	if listErr != nil || len(messages) != 2 || messages[1].Content != "Visible answer" {
		t.Fatalf("messages=%+v err=%v", messages, listErr)
	}
}

func TestRunEngineFailsAfterBoundedEmptyReplyRetries(t *testing.T) {
	fixture := newEngineFixture(t, []fakes.LLMStep{
		{Result: domain.LLMResult{FinishReason: "stop"}},
		{Result: domain.LLMResult{FinishReason: "stop"}},
		{Result: domain.LLMResult{FinishReason: "stop"}},
	}, nil, nil)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunFailed || run.Iterations != 3 || run.Failure == nil || run.Failure.Code != "validation_failed" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	messages, listErr := fixture.store.ListMessages(t.Context(), run.SessionID, 20, nil)
	if listErr != nil || len(messages) != 1 {
		t.Fatalf("messages=%+v err=%v", messages, listErr)
	}
}

func TestRunEngineRetriesIncompleteToolArgumentsBeforeExecution(t *testing.T) {
	spec := domain.ToolSpec{Name: "lookup", JSONSchema: json.RawMessage(`{"type":"object","required":["query"]}`)}
	call := func(id, args string) fakes.LLMStep {
		return fakes.LLMStep{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: id, Name: "lookup", Arguments: json.RawMessage(args)}}, FinishReason: "tool_calls"}}
	}
	tools := &fakes.ToolSet{Tools: []domain.ToolSpec{spec}, Results: []domain.ToolResult{{Content: "result"}}}
	fixture := newEngineFixture(t, []fakes.LLMStep{
		call("missing", `{}`),
		call("complete", `{"query":"weather"}`),
		{Result: domain.LLMResult{Text: "Done", FinishReason: "stop"}},
	}, nil, tools)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || run.Iterations != 3 || len(tools.Calls) != 1 || tools.Calls[0].ID != "complete" {
		t.Fatalf("run=%+v calls=%+v err=%v", run, tools.Calls, err)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 3 || requests[1].Messages[len(requests[1].Messages)-1].Text != toolRetryInstruction {
		t.Fatalf("retry request=%+v", requests)
	}
}

func TestRunEngineFailsAfterBoundedToolResponseRetries(t *testing.T) {
	spec := domain.ToolSpec{Name: "lookup", JSONSchema: json.RawMessage(`{"type":"object","required":["query"]}`)}
	steps := make([]fakes.LLMStep, 3)
	for index := range steps {
		steps[index] = fakes.LLMStep{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: fmt.Sprintf("missing-%d", index), Name: "lookup", Arguments: json.RawMessage(`{}`)}}, FinishReason: "tool_calls"}}
	}
	tools := &fakes.ToolSet{Tools: []domain.ToolSpec{spec}}
	fixture := newEngineFixture(t, steps, nil, tools)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunFailed || run.Iterations != 3 || run.Failure == nil || run.Failure.Code != "validation_failed" || len(tools.Calls) != 0 {
		t.Fatalf("run=%+v calls=%+v err=%v", run, tools.Calls, err)
	}
}

func TestRunEngineStopsSchedulingToolsAtRunBudget(t *testing.T) {
	calls := make([]domain.ToolCall, 0, maxToolCallsPerRun+1)
	for index := 0; index <= maxToolCallsPerRun; index++ {
		calls = append(calls, domain.ToolCall{ID: fmt.Sprintf("call-%d", index), Name: "lookup", Arguments: json.RawMessage(fmt.Sprintf(`{"query":%d}`, index))})
	}
	tools := &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "lookup", JSONSchema: json.RawMessage(`{"type":"object","required":["query"]}`)}}}
	fixture := newEngineFixture(t, []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: calls, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Answer from available context", FinishReason: "stop"}},
	}, nil, tools)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || run.Iterations != 2 || len(tools.Calls) != 0 {
		t.Fatalf("run=%+v calls=%d err=%v", run, len(tools.Calls), err)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 2 || len(requests[1].Tools) != 0 || requests[1].Messages[len(requests[1].Messages)-1].Text != toolCallBudgetInstruction {
		t.Fatalf("final request=%+v", requests)
	}
	messages, listErr := fixture.store.ListMessages(t.Context(), run.SessionID, 20, nil)
	if listErr != nil || len(messages) != 2 {
		t.Fatalf("messages=%+v err=%v", messages, listErr)
	}
}

func TestRunEngineForcesFinalAnswerAfterUsingToolBudget(t *testing.T) {
	calls := make([]domain.ToolCall, 0, maxToolCallsPerRun)
	results := make([]domain.ToolResult, 0, maxToolCallsPerRun)
	for index := 0; index < maxToolCallsPerRun; index++ {
		calls = append(calls, domain.ToolCall{ID: fmt.Sprintf("call-%d", index), Name: "lookup", Arguments: json.RawMessage(fmt.Sprintf(`{"query":%d}`, index))})
		results = append(results, domain.ToolResult{Content: fmt.Sprintf("result-%d", index)})
	}
	tools := &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "lookup", JSONSchema: json.RawMessage(`{"type":"object","required":["query"]}`)}}, Results: results}
	fixture := newEngineFixture(t, []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: calls, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Final answer", FinishReason: "stop"}},
	}, nil, tools)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || len(tools.Calls) != maxToolCallsPerRun {
		t.Fatalf("run=%+v calls=%d err=%v", run, len(tools.Calls), err)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 2 || len(requests[1].Tools) != 0 || requestLastText(requests[1]) != toolCallBudgetInstruction {
		t.Fatalf("final request=%+v", requests)
	}
}

func TestRunEngineNudgesAtIterationBudgetThresholds(t *testing.T) {
	steps := make([]fakes.LLMStep, 0, 9)
	results := make([]domain.ToolResult, 0, 8)
	for iteration := 1; iteration <= 8; iteration++ {
		steps = append(steps, fakes.LLMStep{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: fmt.Sprintf("call-%d", iteration), Name: "lookup", Arguments: json.RawMessage(fmt.Sprintf(`{"query":%d}`, iteration))}}, FinishReason: "tool_calls"}})
		results = append(results, domain.ToolResult{Content: fmt.Sprintf("result-%d", iteration)})
	}
	steps = append(steps, fakes.LLMStep{Result: domain.LLMResult{Text: "Done", FinishReason: "stop"}})
	tools := &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "lookup", JSONSchema: json.RawMessage(`{"type":"object","required":["query"]}`)}}, Results: results}
	fixture := newEngineFixture(t, steps, func(agent *domain.Agent) { agent.MaxIterations = 10 }, tools)
	run, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command)
	if err != nil || run.Status != domain.RunSucceeded || run.Iterations != 9 {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	requests := fixture.llm.Requests()
	if len(requests) != 9 || requestLastText(requests[6]) != wrapUpInstruction || requestLastText(requests[8]) != urgentWrapUpInstruction {
		t.Fatalf("threshold requests=%+v", requests)
	}
}

func TestToolCallsNeedRetryForTruncationMalformedAndMissingRequiredArgs(t *testing.T) {
	spec := domain.ToolSpec{Name: "lookup", JSONSchema: json.RawMessage(`{"type":"object","required":["query"]}`)}
	tests := []struct {
		name   string
		result domain.LLMResult
		want   bool
	}{
		{name: "truncated", result: domain.LLMResult{FinishReason: "MAX_TOKENS", ToolCalls: []domain.ToolCall{{Name: "lookup", Arguments: json.RawMessage(`{"query":"ok"}`)}}}, want: true},
		{name: "malformed", result: domain.LLMResult{FinishReason: "tool_calls", ToolCalls: []domain.ToolCall{{Name: "lookup", Arguments: json.RawMessage(`{"query":`)}}}, want: true},
		{name: "missing required", result: domain.LLMResult{FinishReason: "tool_calls", ToolCalls: []domain.ToolCall{{Name: "lookup", Arguments: json.RawMessage(`{}`)}}}, want: true},
		{name: "complete", result: domain.LLMResult{FinishReason: "tool_calls", ToolCalls: []domain.ToolCall{{Name: "lookup", Arguments: json.RawMessage(`{"query":"ok"}`)}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := toolCallsNeedRetry(test.result, []domain.ToolSpec{spec}); got != test.want {
				t.Fatalf("got=%v want=%v", got, test.want)
			}
		})
	}
}

func requestLastText(request domain.LLMRequest) string {
	if len(request.Messages) == 0 {
		return ""
	}
	return strings.TrimSpace(request.Messages[len(request.Messages)-1].Text)
}
