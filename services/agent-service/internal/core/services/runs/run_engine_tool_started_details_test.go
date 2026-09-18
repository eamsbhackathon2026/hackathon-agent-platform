package runs

import (
	"context"
	"encoding/json"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

// tool.started can only name what an operator opted a parameter into, so what the
// run engine attaches from a call's arguments is the entire trust boundary here.
func TestRunStreamAttachesOnlyTheVisibleParamsToToolStarted(t *testing.T) {
	toolset := &fakes.ToolSet{
		Tools: []domain.ToolSpec{{
			Name: "http_precheck_transfer", DisplayName: "Đang kiểm tra giao dịch",
			VisibleParams: []string{"month"},
			JSONSchema:    json.RawMessage(`{"type":"object"}`),
		}},
		Results: []domain.ToolResult{{Content: "{}"}},
	}
	arguments := json.RawMessage(`{"month":"2026-06","recipient_account":"0123456789"}`)
	steps := []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "http_precheck_transfer", Arguments: arguments}}, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Xong rồi.", FinishReason: "stop"}},
	}
	fixture := newEngineFixture(t, steps, nil, toolset)
	var started domain.RunEvent
	run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
		if event.Type == domain.EventToolStarted {
			started = event
		}
		return nil
	}))
	if err != nil || run.Status != domain.RunSucceeded {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if len(started.Details) != 1 || started.Details[0] != (domain.ToolStartedDetail{Name: "month", Value: "2026-06"}) {
		t.Fatalf("details=%+v", started.Details)
	}
	for _, detail := range started.Details {
		if detail.Name == "recipient_account" {
			t.Fatalf("a parameter with no show_in_progress leaked into tool.started: %+v", started.Details)
		}
	}
}

// A tool with no visible parameter must still emit an empty details array, not a
// nil one, since the wire contract requires the field.
func TestRunStreamEmitsEmptyDetailsWhenNoParamIsVisible(t *testing.T) {
	toolset := &fakes.ToolSet{
		Tools:   []domain.ToolSpec{{Name: "http_get_monthly_summary", DisplayName: "Đang đọc chi tiêu", JSONSchema: json.RawMessage(`{"type":"object"}`)}},
		Results: []domain.ToolResult{{Content: "{}"}},
	}
	steps := []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "http_get_monthly_summary", Arguments: json.RawMessage(`{}`)}}, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Xong rồi.", FinishReason: "stop"}},
	}
	fixture := newEngineFixture(t, steps, nil, toolset)
	var started domain.RunEvent
	if _, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
		if event.Type == domain.EventToolStarted {
			started = event
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if started.Details == nil || len(started.Details) != 0 {
		t.Fatalf("details=%+v", started.Details)
	}
}
