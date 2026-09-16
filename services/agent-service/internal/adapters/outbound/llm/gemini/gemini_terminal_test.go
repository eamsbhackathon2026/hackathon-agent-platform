package gemini

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestStreamRequiresTerminalEvent(t *testing.T) {
	for name, data := range map[string]string{
		"empty":       "",
		"partial":     `{"candidates":[{"content":{"parts":[{"text":"partial"}]}}]}`,
		"tool_call":   `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"weather","args":{"city":"Hue"}}}]}}]}`,
		"unspecified": `{"candidates":[{"finishReason":"FINISH_REASON_UNSPECIFIED"}]}`,
		"feedback":    `{"promptFeedback":{"blockReason":"BLOCKED_REASON_UNSPECIFIED"}}`,
		"usage_only":  `{"usageMetadata":{"promptTokenCount":1}}`,
	} {
		t.Run(name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if data != "" {
					writeSSE(w, data)
				}
			})
			result, err := client.Stream(context.Background(), request(), nil)
			if err != domain.ErrProviderUnreachable || !reflect.DeepEqual(result, domain.LLMResult{}) {
				t.Fatalf("incomplete stream returned result %#v, error %v", result, err)
			}
		})
	}
}

func TestStreamAcceptsBlockedPrompt(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeSSE(w, `{"promptFeedback":{"blockReason":"SAFETY","blockReasonMessage":"private-upstream-detail"},"usageMetadata":{"promptTokenCount":1}}`)
	})
	result, err := client.Stream(context.Background(), request(), nil)
	if err != nil || result.FinishReason != "SAFETY" || result.Text != "" || len(result.ToolCalls) != 0 || len(result.ProviderMeta) != 0 {
		t.Fatalf("blocked prompt returned %#v, error %v", result, err)
	}
}

func TestMalformedModelListReturnsSafeError(t *testing.T) {
	for _, data := range []string{`{"models":"private-invalid-shape"}`, `{"models":["private-invalid-model"]}`, `{invalid-json}`} {
		t.Run(data, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(data))
			})
			models, err := client.ListModels(context.Background())
			if err != domain.ErrProviderBadRequest || models != nil {
				t.Fatalf("malformed model list returned %v, error %v", models, err)
			}
		})
	}
}
