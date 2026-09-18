package runs

import (
	"context"
	"encoding/json"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

// tool.started is the only thing a caller can show a customer while a tool runs, so
// what it carries decides whether that customer reads a sentence or a function name.
func TestRunStreamAnnouncesToolsWithTheirConfiguredLabel(t *testing.T) {
	toolset := &fakes.ToolSet{
		Tools:   []domain.ToolSpec{{Name: "http_get_monthly_summary", DisplayName: "Đang đọc chi tiêu theo tháng", JSONSchema: json.RawMessage(`{"type":"object"}`)}},
		Results: []domain.ToolResult{{Content: "{}"}},
	}
	steps := []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "http_get_monthly_summary", Arguments: json.RawMessage(`{}`)}}, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Tháng này bạn tiêu 12 triệu.", FinishReason: "stop"}},
	}
	fixture := newEngineFixture(t, steps, nil, toolset)
	var started []domain.RunEvent
	run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
		if event.Type == domain.EventToolStarted {
			started = append(started, event)
		}
		return nil
	}))
	if err != nil || run.Status != domain.RunSucceeded {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if len(started) != 1 {
		t.Fatalf("tool.started events=%d", len(started))
	}
	if started[0].DisplayName != "Đang đọc chi tiêu theo tháng" {
		t.Fatalf("emitted label %q", started[0].DisplayName)
	}
	// The provider-facing name still travels: a caller needs it to correlate, it is
	// only the thing not to display.
	if started[0].ToolName != "http_get_monthly_summary" {
		t.Fatalf("tool name %q", started[0].ToolName)
	}
}

// A model can name a tool the agent no longer declares. The label lookup must not
// invent one, and must not stop the run before Execute reports the unknown tool.
func TestRunStreamFallsBackToToolNameWhenNoSpecDeclaresIt(t *testing.T) {
	toolset := &fakes.ToolSet{Results: []domain.ToolResult{{Content: "không có công cụ này", IsError: true}}}
	steps := []fakes.LLMStep{
		{Result: domain.LLMResult{ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "http_missing", Arguments: json.RawMessage(`{}`)}}, FinishReason: "tool_calls"}},
		{Result: domain.LLMResult{Text: "Mình chưa xem được.", FinishReason: "stop"}},
	}
	fixture := newEngineFixture(t, steps, nil, toolset)
	var label string
	if _, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
		if event.Type == domain.EventToolStarted {
			label = event.DisplayName
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if label != "http_missing" {
		t.Fatalf("fallback label %q", label)
	}
}
