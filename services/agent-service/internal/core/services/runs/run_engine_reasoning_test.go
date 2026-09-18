package runs

import (
	"context"
	"encoding/json"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

// A thought summary is not the answer. It reaches the caller as its own event so
// a product can narrate progress, and it must stay out of the reply and out of
// the transcript — otherwise the model's thinking is read back to whoever asked.
func TestRunStreamSendsThoughtSummariesAsTheirOwnEvent(t *testing.T) {
	steps := []fakes.LLMStep{{
		Deltas: []domain.LLMDelta{
			{Reasoning: "Khách hỏi chi tiêu, "},
			{Reasoning: "mình xem giao dịch trước."},
			{Text: "Tháng này bạn tiêu 12 triệu."},
		},
		Result: domain.LLMResult{Text: "Tháng này bạn tiêu 12 triệu.", FinishReason: "stop"},
	}}
	fixture := newEngineFixture(t, steps, func(agent *domain.Agent) { agent.ShowThinking = true }, nil)
	var thoughts, answer []string
	run, err := fixture.service.RunStream(t.Context(), fixture.principal, fixture.command, inbound.RunEventSinkFunc(func(_ context.Context, event domain.RunEvent) error {
		switch event.Type {
		case domain.EventReasoningDelta:
			thoughts = append(thoughts, event.Text)
		case domain.EventMessageDelta:
			answer = append(answer, event.Text)
		}
		return nil
	}))
	if err != nil || run.Status != domain.RunSucceeded {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if len(thoughts) != 2 || thoughts[0] != "Khách hỏi chi tiêu, " {
		t.Fatalf("reasoning events: %#v", thoughts)
	}
	if len(answer) != 1 || answer[0] != "Tháng này bạn tiêu 12 triệu." {
		t.Fatalf("message events: %#v", answer)
	}
	if run.Output == nil || *run.Output != "Tháng này bạn tiêu 12 triệu." {
		t.Fatalf("the answer carries the thinking: %#v", run.Output)
	}
	messages, _ := fixture.store.ListMessages(t.Context(), run.SessionID, 20, nil)
	for _, m := range messages {
		if m.Role == "assistant" && m.Content != "Tháng này bạn tiêu 12 triệu." {
			t.Fatalf("the transcript kept the thinking: %q", m.Content)
		}
	}
}

// The flag is what a product sets; the engine only carries it to the provider.
func TestRunPassesShowThinkingToTheProvider(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		steps := []fakes.LLMStep{{Result: domain.LLMResult{Text: "Xong", FinishReason: "stop"}}}
		fixture := newEngineFixture(t, steps, func(agent *domain.Agent) { agent.ShowThinking = enabled }, &fakes.ToolSet{Tools: []domain.ToolSpec{{Name: "x", JSONSchema: json.RawMessage(`{"type":"object"}`)}}})
		if _, err := fixture.service.RunSync(t.Context(), fixture.principal, fixture.command); err != nil {
			t.Fatal(err)
		}
		if got := fixture.llm.Requests()[0].IncludeThoughts; got != enabled {
			t.Fatalf("agent set %v, provider received %v", enabled, got)
		}
	}
}
