//go:build live

package gemini

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

func TestLiveTwoTurnTool(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-3.8-flash"
	}
	guard, err := netguard.New(netguard.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, err := New(ctx, domain.ProviderConnection{Kind: domain.ProviderGemini, APIKey: key}, guard.NewHTTPClient())
	if err != nil {
		t.Fatal(err)
	}
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatal("list live models:", err)
	}
	if !slices.Contains(models, model) {
		t.Fatal("GEMINI_MODEL is not available for generation")
	}
	req := domain.LLMRequest{
		Model:        model,
		SystemPrompt: "You must call the lookup_weather tool to answer the user. After receiving its result, state the temperature in a short sentence.",
		Messages:     []domain.ChatMessage{{Role: "user", Text: "Use lookup_weather to look up Hanoi's current temperature."}},
		Tools:        []domain.ToolSpec{{Name: "lookup_weather", Description: "Look up the current temperature of a city.", JSONSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`)}},
	}
	first, err := client.Stream(ctx, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) == 0 {
		t.Fatal("model did not call the weather tool")
	}
	results := []domain.ToolResult{}
	for _, call := range first.ToolCalls {
		if call.Name != "lookup_weather" || !json.Valid(call.Arguments) {
			t.Fatal("invalid tool call")
		}
		results = append(results, domain.ToolResult{CallID: call.ID, Name: call.Name, Content: `{"temperature_celsius":29}`})
	}
	req.Messages = append(req.Messages,
		domain.ChatMessage{Role: "assistant", Text: first.Text, ToolCalls: first.ToolCalls, ProviderMeta: first.ProviderMeta},
		domain.ChatMessage{Role: "tool", ToolResults: results},
	)
	second, err := client.Stream(ctx, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Text == "" {
		t.Fatal("model did not answer after the tool result")
	}
	t.Logf("model=%s, models=%d, tool_calls=%d, two-turn replay completed", model, len(models), len(first.ToolCalls))
}
