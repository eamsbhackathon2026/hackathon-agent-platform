//go:build live

package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestLiveToolRoundTrip(t *testing.T) {
	baseURL, key, model := os.Getenv("OPENAI_COMPAT_BASE_URL"), os.Getenv("OPENAI_COMPAT_API_KEY"), os.Getenv("OPENAI_COMPAT_MODEL")
	if baseURL == "" || key == "" || model == "" {
		t.Skip("set OPENAI_COMPAT_BASE_URL, OPENAI_COMPAT_API_KEY and OPENAI_COMPAT_MODEL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, err := New(ctx, domain.ProviderConnection{BaseURL: baseURL, APIKey: key}, &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }})
	if err != nil {
		t.Fatal(err)
	}
	req := domain.LLMRequest{Model: model, SystemPrompt: "Call get_test_value exactly once before answering. After its result, answer with the returned value without another tool call.", Messages: []domain.ChatMessage{{Role: "user", Text: "What is the test value?"}}, Tools: []domain.ToolSpec{{Name: "get_test_value", Description: "Return the test value", JSONSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)}}}
	first, err := client.Stream(ctx, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 1 || first.ToolCalls[0].Name != "get_test_value" {
		t.Fatal("expected one test tool call")
	}
	req.Messages = append(req.Messages, domain.ChatMessage{Role: "assistant", Text: first.Text, ToolCalls: first.ToolCalls}, domain.ChatMessage{Role: "tool", ToolResults: []domain.ToolResult{{CallID: first.ToolCalls[0].ID, Name: "get_test_value", Content: `{"value":"round-trip-ok"}`}}})
	second, err := client.Stream(ctx, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Text == "" || len(second.ToolCalls) != 0 {
		t.Fatal("expected completed text after tool result")
	}
}
