package gemini

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"reflect"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
	"google.golang.org/genai"
)

func TestModelPaginationAndFiltering(t *testing.T) {
	pages := 0
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.Header().Set("Content-Type", "application/json")
		if pages == 1 {
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-a","supportedGenerationMethods":["generateContent"]},{"name":"models/embedding","supportedGenerationMethods":["embedContent"]}],"nextPageToken":"page-two"}`))
			return
		}
		if r.URL.Query().Get("pageToken") != "page-two" {
			t.Error("missing page token")
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-b"}]}`))
	})
	models, err := client.ListModels(context.Background())
	if err != nil || !reflect.DeepEqual(models, []string{"gemini-a", "gemini-b"}) || pages != 2 {
		t.Fatalf("models %v pages %d error %v", models, pages, err)
	}
}

func TestRequestParameters(t *testing.T) {
	req := request()
	req.SystemPrompt = "Be helpful"
	req.Temperature = ptr(0.75)
	req.MaxOutputTokens = ptr(512)
	req.Tools = []domain.ToolSpec{{Name: "weather", Description: "Current weather", JSONSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`)}}
	_, config, err := mapRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if *config.Temperature != 0.75 || config.MaxOutputTokens != 512 || config.SystemInstruction.Parts[0].Text != "Be helpful" || config.Tools[0].FunctionDeclarations[0].ParametersJsonSchema == nil {
		t.Fatalf("config %#v", config)
	}
	for _, temperature := range []float64{math.Inf(1), math.NaN(), math.MaxFloat64, -1} {
		req.Temperature = &temperature
		if _, _, err := mapRequest(req); err != domain.ErrProviderBadRequest {
			t.Fatalf("invalid temperature accepted: %v", temperature)
		}
	}
	req.Temperature = nil
	req.MaxOutputTokens = ptr(int(math.MaxInt32) + 1)
	if _, _, err := mapRequest(req); err != domain.ErrProviderBadRequest {
		t.Fatal("overflow accepted")
	}
}

func TestMetadataRejectsCorruptAndPreservesAuthoritativeParts(t *testing.T) {
	for _, raw := range []string{`garbage`, `{}`, `{"version":2,"parts":[{"text":"a"}]}`, `{"version":1,"parts":[null]}`, `{"version":1,"parts":[{"functionCall":{"name":"weather"}}]}`, `{"version":1,"parts":[{"text":"a"}],"calls":[{"call_id":"x","part_index":2}]}`} {
		_, err := mapMessage(domain.ChatMessage{Role: "assistant", ProviderMeta: []byte(raw)}, map[string]responseBinding{})
		if err != domain.ErrProviderBadRequest {
			t.Errorf("accepted corrupt metadata %s", raw)
		}
	}
	meta := replayMetadata{Version: 1, Parts: []*genai.Part{{Text: "original", ThoughtSignature: []byte("signature")}}}
	raw, _ := json.Marshal(meta)
	content, err := mapMessage(domain.ChatMessage{Role: "assistant", Text: "do not duplicate", ProviderMeta: raw}, map[string]responseBinding{})
	if err != nil || len(content.Parts) != 1 || content.Parts[0].Text != "original" {
		t.Fatalf("authoritative metadata %#v %v", content, err)
	}
}
