package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

// Thinking models reached over an OpenAI-compatible endpoint narrate themselves
// in a field the wire format never defined, and deployments disagree on its name.
// Reading neither name would silently discard the thinking of every such model.
func TestStreamReadsThinkingFromNonStandardFields(t *testing.T) {
	for _, testCase := range []struct{ name, field string }{
		{"the DeepSeek, GLM and Qwen spelling", "reasoning_content"},
		{"the spelling several vLLM builds use", "reasoning"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				serveSSE(w, `{"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"`+testCase.field+`":"Khách hỏi chi tiêu. "}}]}`+"\n"+
					`{"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"`+testCase.field+`":"Xem giao dịch trước."}}]}`+"\n"+
					`{"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Tháng này bạn tiêu 12 triệu."},"finish_reason":"stop"}]}`)
			})
			var answer, thinking string
			var kinds []domain.ReasoningKind
			result, err := client.Stream(context.Background(), domain.LLMRequest{Model: "test"}, func(d domain.LLMDelta) {
				answer += d.Text
				if d.Reasoning != "" {
					thinking += d.Reasoning
					kinds = append(kinds, d.ReasoningKind)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			// Reasoning over this path is raw, not a summary written to be shown.
			for _, kind := range kinds {
				if kind != domain.ReasoningRaw {
					t.Fatalf("reasoning kind: %q", kind)
				}
			}
			if thinking != "Khách hỏi chi tiêu. Xem giao dịch trước." {
				t.Fatalf("thinking: %q", thinking)
			}
			// Thinking must stay out of the answer, in the stream and in the result.
			if answer != "Tháng này bạn tiêu 12 triệu." || result.Text != answer {
				t.Fatalf("the answer carries the thinking: %q / %q", answer, result.Text)
			}
		})
	}
}

// A model that says nothing about its thinking must not produce empty deltas.
func TestStreamWithoutThinkingFieldsEmitsNothingExtra(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		serveSSE(w, `{"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Xong"},"finish_reason":"stop"}]}`)
	})
	var count int
	if _, err := client.Stream(context.Background(), domain.LLMRequest{Model: "test"}, func(d domain.LLMDelta) {
		if d.Reasoning != "" {
			count++
		}
	}); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("emitted %d reasoning deltas from a model that narrates nothing", count)
	}
}

var _ = json.Valid
