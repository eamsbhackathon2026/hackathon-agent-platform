//go:build integration && live

package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestLiveGeminiRunStreamAndTrace(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-3.8-flash"
	}

	server, pool := identityTestServer(t)
	c := newCatalogHTTP(t, server)
	var provider gen.Provider
	c.request("POST", "/v1/providers", map[string]any{"name": "Live run Gemini", "kind": "gemini", "api_key": key, "default_model": model}, 201, &provider)
	var agent gen.Agent
	c.request("POST", "/v1/agents", map[string]any{"name": "Live run assistant", "provider_id": provider.Id, "model": model}, 201, &agent)

	response, err := runRequest(server.Client(), server.URL+"/v1/agents/"+agent.Id.String()+"/runs/stream", c.token, "", map[string]any{"input": map[string]string{"message": "Trả lời ngắn gọn bằng tiếng Việt: 2 + 2 bằng bao nhiêu?"}})
	if err != nil {
		t.Fatal(err)
	}
	stream, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != 200 || !bytes.Contains(stream, []byte("event: message.delta")) || !bytes.Contains(stream, []byte("event: run.completed")) {
		t.Fatalf("stream status=%d body=%s err=%v", response.StatusCode, stream, readErr)
	}

	var runID uuid.UUID
	if err = pool.QueryRow(t.Context(), "SELECT id FROM runs ORDER BY created_at DESC,id DESC LIMIT 1").Scan(&runID); err != nil {
		t.Fatal(err)
	}
	var spans gen.SpanPage
	c.request("GET", "/v1/runs/"+runID.String()+"/spans", nil, 200, &spans)
	for _, span := range spans.Items {
		if span.Kind == gen.SpanKind("llm_call") && !span.Usage.InputTokens.IsNull() && !span.Usage.OutputTokens.IsNull() && span.DurationMs >= 0 {
			t.Logf("live Gemini SSE and trace verified; model=%s", model)
			return
		}
	}
	t.Fatalf("live Gemini trace lacks LLM usage: %+v", spans.Items)
}

func TestLiveGeminiHTTPToolRun(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-3.8-flash"
	}
	var calls atomic.Int32
	toolServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.URL.Query().Get("subject") != "meaning" || request.Header.Get("Authorization") != "Bearer live-tool-fixture" {
			t.Errorf("tool request query=%q auth=%q", request.URL.RawQuery, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"answer":42}`)
	}))
	defer toolServer.Close()

	server, _ := identityTestServer(t)
	c := newCatalogHTTP(t, server)
	var provider gen.Provider
	c.request("POST", "/v1/providers", map[string]any{"name": "Live tool Gemini", "kind": "gemini", "api_key": key, "default_model": model}, 201, &provider)
	var agent gen.Agent
	c.request("POST", "/v1/agents", map[string]any{"name": "Live tool assistant", "provider_id": provider.Id, "model": model, "system_prompt": "You must call http_known_fact exactly once before answering. Use subject=meaning. Then answer briefly in Vietnamese using the returned number."}, 201, &agent)
	var tool gen.HttpTool
	c.request("POST", "/v1/tools", map[string]any{"slug": "known_fact", "display_name": "Known fact", "description": "Return a known numeric fact", "method": "GET", "url_template": toolServer.URL, "params": []map[string]any{{"name": "subject", "type": "string", "description": "Fact subject", "required": true, "in": "query"}}, "secret_headers": map[string]string{"Authorization": "Bearer live-tool-fixture"}}, 201, &tool)
	c.request("PUT", "/v1/agents/"+agent.Id.String()+"/tools", map[string]any{"tool_ids": []string{tool.Id.String()}, "mcp_server_ids": []string{}}, 200, nil)
	var run gen.Run
	c.request("POST", "/v1/agents/"+agent.Id.String()+"/runs", map[string]any{"input": map[string]string{"message": "Hãy dùng công cụ để tìm con số rồi trả lời."}, "mode": "sync"}, 200, &run)
	if run.Status != gen.RunStatusSucceeded || run.Output.IsNull() || run.Output.MustGet() == "" || calls.Load() != 1 {
		t.Fatalf("live tool run=%+v calls=%d", run, calls.Load())
	}
	var spans gen.SpanPage
	c.request("GET", "/v1/runs/"+run.Id.String()+"/spans", nil, 200, &spans)
	for _, span := range spans.Items {
		if span.Kind == gen.SpanKindToolCall && span.ToolName.MustGet() == "http_known_fact" {
			t.Logf("live Gemini HTTP tool verified; model=%s calls=%d", model, calls.Load())
			return
		}
	}
	t.Fatalf("live Gemini run lacks tool span: %+v", spans.Items)
}
