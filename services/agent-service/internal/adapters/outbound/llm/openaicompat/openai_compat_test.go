package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := New(context.Background(), domain.ProviderConnection{BaseURL: server.URL + "/v1", APIKey: "test-key"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func serveSSE(w http.ResponseWriter, chunks string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, line := range strings.Split(strings.TrimSpace(chunks), "\n") {
		if line != "" {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
		}
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
}

func TestParallelStreamAndMapping(t *testing.T) {
	// Synthetic fixture follows the official Chat Completions SSE wire format:
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events
	fixture, err := os.ReadFile("testdata/parallel.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("wrong request destination/auth")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["temperature"] != float64(0) || body["max_completion_tokens"] != float64(1) {
			t.Errorf("optional values lost: %v", body)
		}
		messages := body["messages"].([]any)
		if len(messages) != 5 || messages[4].(map[string]any)["tool_call_id"] != "old" {
			t.Errorf("incorrect messages: %v", messages)
		}
		serveSSE(w, string(fixture))
	})
	zero, one := 0.0, 1
	req := domain.LLMRequest{Model: "test", SystemPrompt: "system", Temperature: &zero, MaxOutputTokens: &one, Messages: []domain.ChatMessage{{Role: "system", Text: "extra"}, {Role: "user", Text: "hello"}, {Role: "assistant", ToolCalls: []domain.ToolCall{{ID: "old", Name: "first", Arguments: json.RawMessage(`{}`)}}}, {Role: "tool", ToolResults: []domain.ToolResult{{CallID: "old", Content: "ok"}}}}, Tools: []domain.ToolSpec{{Name: "first", JSONSchema: json.RawMessage(`{"type":"object"}`)}}}
	var text string
	result, err := client.Stream(context.Background(), req, func(d domain.LLMDelta) { text += d.Text })
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hello world" || result.Text != text || result.FinishReason != "tool_calls" || len(result.ToolCalls) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.ToolCalls[0].ID != "call_a" || string(result.ToolCalls[0].Arguments) != `{"a":1}` || result.ToolCalls[1].ID != "call_b" || string(result.ToolCalls[1].Arguments) != `{"b":2}` {
		t.Fatalf("calls: %+v", result.ToolCalls)
	}
	if result.Usage.InputTokens == nil || *result.Usage.InputTokens != 12 || result.Usage.OutputTokens == nil || *result.Usage.OutputTokens != 8 {
		t.Fatalf("usage: %+v", result.Usage)
	}
}

func TestUsageAndCollection(t *testing.T) {
	for _, usage := range []string{"", `,"usage":null`, `,"usage":{"prompt_tokens":0}`} {
		t.Run(usage, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				serveSSE(w, `{"id":"x","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]`+usage+`}`)
			})
			result, err := client.Stream(context.Background(), domain.LLMRequest{Model: "test"}, nil)
			if err != nil || result.Text != "ok" {
				t.Fatalf("%+v %v", result, err)
			}
			if result.Usage.OutputTokens != nil {
				t.Fatal("invented output usage")
			}
			if strings.Contains(usage, "prompt_tokens") {
				if result.Usage.InputTokens == nil || *result.Usage.InputTokens != 0 {
					t.Fatal("lost zero")
				}
			} else if result.Usage.InputTokens != nil {
				t.Fatal("invented input usage")
			}
		})
	}
}

func TestStreamFailures(t *testing.T) {
	for name, body := range map[string]string{"empty": "", "malformed": "{", "truncated": `{"id":"x","choices":[{"index":0,"delta":{"content":"partial"}}]}`, "bad_arguments": `{"id":"x","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"a","type":"function","function":{"name":"x","arguments":"{"}}]},"finish_reason":"tool_calls"}]}`, "bad_index": `{"id":"x","choices":[{"index":-1,"delta":{},"finish_reason":"stop"}]}`} {
		t.Run(name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) { serveSSE(w, body) })
			result, err := client.Stream(context.Background(), domain.LLMRequest{Model: "test"}, nil)
			if !errors.Is(err, domain.ErrProviderUnreachable) || len(result.ToolCalls) != 0 {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
}

func TestErrorMappingNoRetries(t *testing.T) {
	for status, want := range map[int]error{401: domain.ErrProviderAuth, 403: domain.ErrProviderAuth, 404: domain.ErrModelNotFound, 429: domain.ErrProviderRateLimited, 400: domain.ErrProviderBadRequest, 422: domain.ErrProviderBadRequest, 503: domain.ErrProviderUnreachable} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var count atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				count.Add(1)
				w.WriteHeader(status)
				_, _ = fmt.Fprint(w, `{"error":{"message":"secret body","type":"error"}}`)
			})
			_, err := client.Stream(context.Background(), domain.LLMRequest{Model: "test"}, nil)
			if !errors.Is(err, want) || count.Load() != 1 {
				t.Fatalf("%v requests=%d", err, count.Load())
			}
		})
	}
}

func TestListModelsAndUnsupported(t *testing.T) {
	for _, status := range []int{200, 404, 405, 501} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/models" {
					t.Error(r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = fmt.Fprint(w, `{"object":"list","data":[{"id":"b"},{"id":"a"}]}`)
			})
			ids, err := client.ListModels(context.Background())
			if status == 200 {
				if err != nil || len(ids) != 2 {
					t.Fatalf("%v %v", ids, err)
				}
			} else if !errors.Is(err, domain.ErrModelsUnsupported) {
				t.Fatal(err)
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"x\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := client.Stream(ctx, domain.LLMRequest{Model: "test"}, func(domain.LLMDelta) { cancel() })
	if !errors.Is(err, context.Canceled) || result.Text != "" {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestInvalidRequestsDoNotReachProvider(t *testing.T) {
	zero := 0
	for name, req := range map[string]domain.LLMRequest{
		"empty_model":       {},
		"zero_tokens":       {Model: "test", MaxOutputTokens: &zero},
		"invalid_schema":    {Model: "test", Tools: []domain.ToolSpec{{Name: "x", JSONSchema: json.RawMessage(`[]`)}}},
		"null_schema":       {Model: "test", Tools: []domain.ToolSpec{{Name: "x", JSONSchema: json.RawMessage(`null`)}}},
		"invalid_arguments": {Model: "test", Messages: []domain.ChatMessage{{Role: "assistant", ToolCalls: []domain.ToolCall{{ID: "a", Name: "x", Arguments: json.RawMessage(`{`)}}}}},
		"unknown_role":      {Model: "test", Messages: []domain.ChatMessage{{Role: "unknown"}}},
	} {
		t.Run(name, func(t *testing.T) {
			client := testClient(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid request reached provider") })
			_, err := client.Stream(context.Background(), req, nil)
			if !errors.Is(err, domain.ErrProviderBadRequest) {
				t.Fatal(err)
			}
		})
	}
}

func TestNewRequiresTransportAndPreservesCancellation(t *testing.T) {
	if _, err := New(context.Background(), domain.ProviderConnection{BaseURL: "https://example.com/v1"}, nil); !errors.Is(err, domain.ErrProviderNotConfigured) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(ctx, domain.ProviderConnection{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	wrapped := fmt.Errorf("https://secret.example: %w", context.Canceled)
	if providerError(context.Background(), wrapped, false) != context.Canceled {
		t.Fatal("context error not sanitized")
	}
}
