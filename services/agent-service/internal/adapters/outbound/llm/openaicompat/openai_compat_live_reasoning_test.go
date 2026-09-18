//go:build live

package openaicompat

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
)

// TestLiveReasoningFields answers one question about a deployment: does this
// model narrate its own thinking, and under which field name? Reading the code
// cannot answer it, and guessing wrong costs a feature that looks implemented and
// never fires. Run it against the real endpoint:
//
//	OPENAI_COMPAT_BASE_URL=https://maas-llm-aiplatform-hcm.api.vngcloud.vn/v1 \
//	OPENAI_COMPAT_API_KEY=... OPENAI_COMPAT_MODEL=z-ai/glm-5.2-hackathon \
//	go test -tags live -run TestLiveReasoningFields -v ./internal/adapters/outbound/llm/openaicompat/
func TestLiveReasoningFields(t *testing.T) {
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

	// Report every unknown field the deployment sends, not only the two names the
	// adapter reads, so a third spelling shows up here instead of staying invisible.
	seen := map[string]string{}
	debugExtras = func(fields map[string]string) {
		for name, raw := range fields {
			if _, known := seen[name]; !known {
				seen[name] = raw
			}
		}
	}
	defer func() { debugExtras = nil }()

	var answer, thinking strings.Builder
	result, err := client.Stream(ctx, domain.LLMRequest{
		Model:        model,
		SystemPrompt: "Trả lời bằng tiếng Việt.",
		Messages: []domain.ChatMessage{{Role: "user", Text: "Con đầu 12 tuổi, con thứ hai bằng nửa tuổi con đầu, " +
			"con út kém con thứ hai 2 tuổi. Tổng tuổi ba đứa là bao nhiêu? Nghĩ từng bước rồi trả lời."}},
	}, func(d domain.LLMDelta) {
		answer.WriteString(d.Text)
		thinking.WriteString(d.Reasoning)
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("unknown delta fields: %v", seen)
	t.Logf("answer: %s", result.Text)
	if thinking.Len() == 0 {
		t.Logf("NO THINKING: this model sends none, so a product has only tool calls to narrate")
		return
	}
	t.Logf("THINKING (%d chars): %s", thinking.Len(), thinking.String())
	if strings.Contains(result.Text, thinking.String()) {
		t.Fatal("the answer repeats the thinking: the adapter is mixing the two")
	}
}
