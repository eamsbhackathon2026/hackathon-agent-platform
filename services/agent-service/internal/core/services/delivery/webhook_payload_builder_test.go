package delivery_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/services/delivery"
)

func TestBuildPayloadUsesPublicSnakeCaseShape(t *testing.T) {
	now := time.Date(2026, 9, 15, 1, 2, 3, 0, time.UTC)
	inputTokens := 7
	run := domain.Run{ID: uuid.New(), AgentID: uuid.New(), SessionID: uuid.New(), Mode: domain.RunModeAsync, Status: domain.RunFailed, Source: domain.RunSourceAPI, Input: domain.RunInput{Message: "hi"}, Failure: &domain.RunFailure{Code: "interrupted", Message: "stopped"}, Usage: domain.TokenUsage{InputTokens: &inputTokens}, Metadata: json.RawMessage(`{}`), CreatedAt: now, FinishedAt: &now}
	raw, err := delivery.BuildPayload(run, now)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		t.Fatal("payload không phải JSON")
	}
	data := payload["data"].(map[string]any)
	usage := data["usage"].(map[string]any)
	failure := data["error"].(map[string]any)
	if payload["type"] != "run.failed" || data["agent_id"] != run.AgentID.String() || usage["input_tokens"] != float64(7) || failure["code"] != "interrupted" {
		t.Fatalf("payload=%s", raw)
	}
	if _, exists := data["AgentID"]; exists {
		t.Fatal("payload làm lộ field Go")
	}
}

func TestBuildPayloadCarriesStructuredToolResults(t *testing.T) {
	run := domain.Run{ID: uuid.New(), AgentID: uuid.New(), SessionID: uuid.New(), Mode: domain.RunModeAsync, Status: domain.RunSucceeded, Source: domain.RunSourceAPI, Input: domain.RunInput{Message: "hi"}, Metadata: json.RawMessage(`{}`),
		ToolResults: []domain.RunToolResult{{CallID: "call_1", ToolName: "http_precheck_transfer", Arguments: json.RawMessage(`{"customer_id":1}`), Result: json.RawMessage(`{"level":"intervene","scenario_id":"S03"}`)}}}
	raw, err := delivery.BuildPayload(run, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Data struct {
			ToolResults []map[string]any `json:"tool_results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.ToolResults) != 1 {
		t.Fatalf("payload=%s", raw)
	}
	item := payload.Data.ToolResults[0]
	result, _ := item["result"].(map[string]any)
	if item["tool_name"] != "http_precheck_transfer" || item["is_error"] != false || result["level"] != "intervene" {
		t.Fatalf("tool_results sai: %s", raw)
	}
	// A run without tool calls still exposes an array, never null, so consumers can range over it.
	raw, _ = delivery.BuildPayload(domain.Run{ID: uuid.New(), Metadata: json.RawMessage(`{}`), Status: domain.RunSucceeded}, time.Now())
	if !json.Valid(raw) || !strings.Contains(string(raw), `"tool_results":[]`) {
		t.Fatalf("run không có tool phải trả mảng rỗng: %s", raw)
	}
}
