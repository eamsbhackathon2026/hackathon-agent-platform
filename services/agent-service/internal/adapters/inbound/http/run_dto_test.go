package http

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

// Integrating systems branch on tool_results, so its wire shape is part of the contract:
// null when the transcript was not loaded, an array otherwise, results as real JSON values.
func TestRunDTOExposesToolResultsAsStructuredJSON(t *testing.T) {
	run := domain.Run{Metadata: json.RawMessage(`{}`), ToolResults: []domain.RunToolResult{
		{CallID: "call_1", ToolName: "http_precheck_transfer", Arguments: json.RawMessage(`{"customer_id":100001,"session_flags":{"screen_sharing":true}}`), Result: json.RawMessage(`{"score":76,"level":"intervene","scenario_id":"S09"}`)},
		{CallID: "call_2", ToolName: "http_get_customer", Arguments: json.RawMessage(`{}`), Result: json.RawMessage(`"Không kết nối được tới hệ thống đích."`), IsError: true},
	}}
	dto, err := runDTO(run)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		ToolResults []struct {
			CallID    string         `json:"call_id"`
			ToolName  string         `json:"tool_name"`
			Arguments map[string]any `json:"arguments"`
			Result    any            `json:"result"`
			IsError   bool           `json:"is_error"`
		} `json:"tool_results"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.ToolResults) != 2 {
		t.Fatalf("tool_results=%s", raw)
	}
	first := decoded.ToolResults[0]
	result, _ := first.Result.(map[string]any)
	flags, _ := first.Arguments["session_flags"].(map[string]any)
	if first.ToolName != "http_precheck_transfer" || result["level"] != "intervene" || result["score"] != float64(76) || flags["screen_sharing"] != true {
		t.Fatalf("kết quả đầu không phải JSON có cấu trúc: %s", raw)
	}
	if second := decoded.ToolResults[1]; !second.IsError || second.Result != "Không kết nối được tới hệ thống đích." {
		t.Fatalf("kết quả lỗi phải là chuỗi JSON: %s", raw)
	}
}

func TestRunDTOKeepsUnloadedToolResultsNull(t *testing.T) {
	dto, err := runDTO(domain.Run{Metadata: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(dto)
	if !strings.Contains(string(raw), `"tool_results":null`) {
		t.Fatalf("run chưa tải transcript phải trả null, nhận: %s", raw)
	}
	dto, _ = runDTO(domain.Run{Metadata: json.RawMessage(`{}`), ToolResults: []domain.RunToolResult{}})
	raw, _ = json.Marshal(dto)
	if !strings.Contains(string(raw), `"tool_results":[]`) {
		t.Fatalf("run không gọi tool phải trả mảng rỗng, nhận: %s", raw)
	}
}
