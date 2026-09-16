package runs

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestEstimateLLMInputCountsCompleteRequest(t *testing.T) {
	base := estimateLLMInput(domain.LLMRequest{SystemPrompt: "system", Messages: []domain.ChatMessage{{Role: "user", Text: "hello"}}})
	complete := estimateLLMInput(domain.LLMRequest{
		SystemPrompt: strings.Repeat("s", 30),
		Tools:        []domain.ToolSpec{{Name: "lookup", Description: "Lookup data", JSONSchema: json.RawMessage(`{"type":"object"}`)}},
		Messages: []domain.ChatMessage{
			{Role: "assistant", Text: "calling", ProviderMeta: []byte(`{"opaque":true}`), ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "lookup", Arguments: json.RawMessage(`{"q":"value"}`), ProviderMeta: []byte(`{"signature":"x"}`)}}},
			{Role: "tool", ToolResults: []domain.ToolResult{{CallID: "call-1", Name: "lookup", Content: strings.Repeat("result", 20)}}},
		},
	})
	if complete <= base {
		t.Fatalf("complete estimate=%d base=%d", complete, base)
	}
}

func TestContextBudgetCalibrationOnlyBecomesMoreConservative(t *testing.T) {
	estimated, actual := 100, 250
	budgeter := newContextBudgeter(domain.Session{LastPromptEstimatedTokens: &estimated, LastPromptTokens: &actual})
	agent := domain.Agent{ContextWindowTokens: domain.MinContextWindowTokens}
	budget := budgeter.assess(agent, []domain.ChatMessage{{Role: "user", Text: strings.Repeat("x", 300)}}, nil)
	if budget.adjustedEstimate != int(float64(budget.rawEstimate)*2.5) {
		t.Fatalf("budget=%+v", budget)
	}
	budgeter.observe(100, 120)
	if budgeter.calibrationFactor != 2.5 {
		t.Fatalf("calibration factor decreased: %f", budgeter.calibrationFactor)
	}
}

func TestContextBudgetUsesTheEffectiveProviderOutputCap(t *testing.T) {
	agent := domain.Agent{ContextWindowTokens: domain.MinContextWindowTokens}
	budget := newContextBudgeter(domain.Session{}).assess(agent, []domain.ChatMessage{{Role: "user", Text: "hello"}}, nil)
	want := domain.MinContextWindowTokens - domain.EffectiveMaxOutputTokens(agent) - domain.ContextSafetyTokens(domain.MinContextWindowTokens)
	if budget.hardInputLimit != want {
		t.Fatalf("hard input limit=%d want=%d", budget.hardInputLimit, want)
	}
}

func TestPruneOldToolResultsKeepsRecentExchangesAndInputImmutable(t *testing.T) {
	messages := []domain.ChatMessage{}
	for index := 0; index < 5; index++ {
		messages = append(messages, domain.ChatMessage{Role: "tool", ToolResults: []domain.ToolResult{{CallID: "call", Name: "lookup", Content: "result"}}})
	}
	pruned := pruneOldToolResults(messages)
	if pruned[0].ToolResults[0].Content != prunedToolResultContent || pruned[1].ToolResults[0].Content != prunedToolResultContent {
		t.Fatalf("old results were not pruned: %+v", pruned)
	}
	for index := 2; index < len(pruned); index++ {
		if pruned[index].ToolResults[0].Content != "result" {
			t.Fatalf("recent result %d was pruned", index)
		}
	}
	if messages[0].ToolResults[0].Content != "result" {
		t.Fatal("source history was mutated")
	}
}
