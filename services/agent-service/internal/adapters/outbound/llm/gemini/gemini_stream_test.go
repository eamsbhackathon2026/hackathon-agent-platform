package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"google.golang.org/genai"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := New(context.Background(), domain.ProviderConnection{Kind: domain.ProviderGemini, APIKey: "fixture-key", BaseURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func writeSSE(w http.ResponseWriter, lines string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, line := range strings.Split(strings.TrimSpace(lines), "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
	}
}

func request() domain.LLMRequest {
	return domain.LLMRequest{Model: "fixture-model", Messages: []domain.ChatMessage{{Role: "user", Text: "Weather?"}}}
}

func TestToolCallsAndSignaturesRoundTrip(t *testing.T) {
	fixture, err := os.ReadFile("testdata/tool-turn.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, fullMessage := range []bool{true, false} {
		t.Run(fmt.Sprintf("full_message_%v", fullMessage), func(t *testing.T) {
			var requests []struct {
				Contents []*genai.Content `json:"contents"`
			}
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Contents []*genai.Content `json:"contents"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				requests = append(requests, body)
				if len(requests) == 1 {
					writeSSE(w, string(fixture))
					return
				}
				writeSSE(w, `{"candidates":[{"content":{"parts":[{"text":"Done"}]},"finishReason":"STOP"}]}`)
			})
			var deltas, thoughts []string
			first, err := client.Stream(context.Background(), request(), func(delta domain.LLMDelta) {
				if delta.Reasoning != "" {
					if delta.ReasoningKind != domain.ReasoningSummary {
						t.Fatalf("reasoning kind: %q", delta.ReasoningKind)
					}
					thoughts = append(thoughts, delta.Reasoning)
					return
				}
				deltas = append(deltas, delta.Text)
			})
			if err != nil {
				t.Fatal(err)
			}
			if first.Text != "Checking weather." || !reflect.DeepEqual(deltas, []string{"Checking ", "weather."}) {
				t.Fatalf("public output: %#v %#v", first.Text, deltas)
			}
			// A thought part travels separately and never joins the answer, or the
			// model's private reasoning would be read out to whoever asked.
			if !reflect.DeepEqual(thoughts, []string{"private reasoning"}) {
				t.Fatalf("thought summaries: %#v", thoughts)
			}
			if strings.Contains(first.Text, "private reasoning") {
				t.Fatalf("thought leaked into the answer: %q", first.Text)
			}
			if len(first.ToolCalls) != 3 || first.ToolCalls[0].ID == "" || first.ToolCalls[0].ID == first.ToolCalls[1].ID || first.ToolCalls[2].ID != "provided-id" {
				t.Fatalf("calls: %#v", first.ToolCalls)
			}
			if *first.Usage.InputTokens != 5 || *first.Usage.OutputTokens != 8 || first.FinishReason != "STOP" {
				t.Fatalf("result: %#v", first)
			}
			assistant := domain.ChatMessage{Role: "assistant", Text: first.Text, ToolCalls: first.ToolCalls}
			if fullMessage {
				assistant.ProviderMeta = first.ProviderMeta
			}
			results := []domain.ToolResult{}
			for index, call := range first.ToolCalls {
				results = append(results, domain.ToolResult{CallID: call.ID, Name: call.Name, Content: fmt.Sprint(index), IsError: index == 1})
			}
			next := request()
			next.Messages = append(next.Messages, assistant, domain.ChatMessage{Role: "tool", ToolResults: results})
			second, err := client.Stream(context.Background(), next, nil)
			if err != nil || second.Text != "Done" {
				t.Fatalf("second turn: %v %#v", err, second)
			}
			replayed := requests[1].Contents[1]
			if replayed.Role != "model" {
				t.Fatalf("role: %s", replayed.Role)
			}
			if fullMessage {
				meta, err := readMetadata(first.ProviderMeta)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(replayed.Parts, meta.Parts) {
					t.Fatalf("ordered parts changed: %#v", replayed.Parts)
				}
			}
			var calls []*genai.Part
			for _, part := range replayed.Parts {
				if part.FunctionCall != nil {
					calls = append(calls, part)
				}
			}
			if len(calls) != 3 || calls[0].FunctionCall.ID != "" || calls[1].FunctionCall.ID != "" || string(calls[0].ThoughtSignature) != "call-one" || string(calls[1].ThoughtSignature) != "call-two" {
				t.Fatalf("call metadata lost: %#v", calls)
			}
			responseParts := requests[1].Contents[2].Parts
			if len(responseParts) != 3 || responseParts[0].FunctionResponse.ID != "" || responseParts[1].FunctionResponse.ID != "" || responseParts[2].FunctionResponse.ID != "provided-id" || responseParts[1].FunctionResponse.Response["error"] != "1" {
				t.Fatalf("tool result IDs or output incorrect: %#v", responseParts)
			}
		})
	}
}

func TestUsageMissingZeroAndLatest(t *testing.T) {
	for _, tc := range []struct {
		name, lines   string
		input, output *int
	}{
		{name: "missing", lines: `{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`},
		{name: "zero", lines: `{"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0}}`, input: new(int), output: new(int)},
		{name: "latest", lines: "{\"usageMetadata\":{\"promptTokenCount\":4,\"candidatesTokenCount\":2}}\n{\"usageMetadata\":{\"promptTokenCount\":4,\"candidatesTokenCount\":7}}", input: ptr(4), output: ptr(7)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				writeSSE(w, tc.lines+"\n"+`{"candidates":[{"finishReason":"STOP"}]}`)
			})
			result, err := client.Stream(context.Background(), request(), nil)
			if err != nil || !reflect.DeepEqual(result.Usage, domain.TokenUsage{InputTokens: tc.input, OutputTokens: tc.output}) {
				t.Fatalf("usage: %#v %v", result.Usage, err)
			}
		})
	}
}

func ptr[T any](value T) *T { return &value }
