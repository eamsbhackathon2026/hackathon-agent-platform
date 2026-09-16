package domain

import (
	"encoding/json"
	"testing"
)

func TestToolResultsFromMessagesPairsCallsWithResultsInOrder(t *testing.T) {
	precheck, questions := "call_1", "call_2"
	messages := []Message{
		{Role: "user", Content: "chuyển tiền"},
		{Role: "assistant", Content: "Để tôi kiểm tra.", ToolCalls: []ToolCall{
			{ID: precheck, Name: "http_precheck_transfer", Arguments: json.RawMessage(`{"customer_id":100001,"session_flags":{"screen_sharing":true}}`)},
			{ID: questions, Name: "http_get_questions", Arguments: json.RawMessage(`{"scenario_id":"S03"}`)},
		}},
		{Role: "tool", ToolCallID: &precheck, ToolName: strPtr("http_precheck_transfer"), Content: `{"score":61,"level":"intervene"}`},
		{Role: "tool", ToolCallID: &questions, ToolName: strPtr("http_get_questions"), Content: "Không kết nối được tới hệ thống đích.", IsError: true},
		{Role: "assistant", Content: "Tôi cần hỏi thêm."},
	}
	results := ToolResultsFromMessages(messages)
	if len(results) != 2 {
		t.Fatalf("số kết quả=%d", len(results))
	}
	first := results[0]
	if first.CallID != precheck || first.ToolName != "http_precheck_transfer" || first.IsError {
		t.Fatalf("kết quả đầu sai: %+v", first)
	}
	var arguments map[string]any
	if err := json.Unmarshal(first.Arguments, &arguments); err != nil || arguments["customer_id"] != float64(100001) {
		t.Fatalf("đối số không giữ nguyên: %s", first.Arguments)
	}
	var body map[string]any
	if err := json.Unmarshal(first.Result, &body); err != nil || body["level"] != "intervene" {
		t.Fatalf("kết quả JSON phải được giữ nguyên dạng JSON: %s", first.Result)
	}
	second := results[1]
	if !second.IsError || string(second.Result) != `"Không kết nối được tới hệ thống đích."` {
		t.Fatalf("kết quả không phải JSON phải được bọc thành chuỗi JSON: %+v", second)
	}
}

func TestToolResultsFromMessagesHandlesOrphanToolMessage(t *testing.T) {
	orphan := "call_x"
	results := ToolResultsFromMessages([]Message{{Role: "tool", ToolCallID: &orphan, ToolName: strPtr("http_lookup"), Content: `[1,2]`}})
	if len(results) != 1 || results[0].ToolName != "http_lookup" || string(results[0].Arguments) != `{}` || string(results[0].Result) != `[1,2]` {
		t.Fatalf("kết quả mồ côi sai: %+v", results)
	}
	if got := ToolResultsFromMessages(nil); len(got) != 0 || got == nil {
		t.Fatalf("không có tool call phải trả mảng rỗng, không phải nil: %#v", got)
	}
}

func strPtr(value string) *string { return &value }
