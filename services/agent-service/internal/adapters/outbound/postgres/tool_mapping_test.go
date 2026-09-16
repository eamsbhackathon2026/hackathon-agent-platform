package postgres

import (
	"encoding/json"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestToolParamsSurviveEncodeDecode(t *testing.T) {
	params := []domain.ToolParam{
		{Name: "customer_id", Type: domain.ToolParamInteger, Description: "Mã CIF", Required: true, Location: domain.ToolParamBody},
		{Name: "session_flags", Type: domain.ToolParamObject, Description: "Cờ phiên", Location: domain.ToolParamBody, Fields: []domain.ToolParamField{
			{Name: "screen_sharing", Type: domain.ToolParamBoolean, Description: "Đang chia sẻ màn hình", Required: true},
			{Name: "device_age_days", Type: domain.ToolParamInteger, Description: "Số ngày dùng thiết bị"},
		}},
		{Name: "recent_events", Type: domain.ToolParamArray, Description: "Mã sự kiện", Location: domain.ToolParamBody, ItemType: domain.ToolParamString},
	}
	decoded, err := decodeToolParams(encodeToolParams(params))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(params) {
		t.Fatalf("số tham số=%d", len(decoded))
	}
	flags := decoded[1]
	if len(flags.Fields) != 2 || flags.Fields[0].Name != "screen_sharing" || !flags.Fields[0].Required || flags.Fields[1].Type != domain.ToolParamInteger {
		t.Fatalf("trường lồng nhau mất khi lưu: %+v", flags)
	}
	if decoded[2].ItemType != domain.ToolParamString {
		t.Fatalf("kiểu phần tử mất khi lưu: %+v", decoded[2])
	}
	if len(decoded[0].Fields) != 0 {
		t.Fatalf("tham số nguyên thủy không nên có trường lồng: %+v", decoded[0])
	}
}

func TestToolParamsDecodeRowsSavedBeforeNestedShapes(t *testing.T) {
	// Rows written before nested declarations existed carry neither key.
	legacy := []byte(`[{"name":"q","type":"string","description":"Từ khóa","required":true,"in":"query"}]`)
	decoded, err := decodeToolParams(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].ItemType != "" || len(decoded[0].Fields) != 0 {
		t.Fatalf("giải mã bản ghi cũ sai: %+v", decoded)
	}
}

func TestEncodedToolParamsOmitEmptyNestedKeys(t *testing.T) {
	// Keeping the stored JSON clean avoids noisy diffs on rows that declare no shape.
	encoded := encodeToolParams([]domain.ToolParam{{Name: "q", Type: domain.ToolParamString, Description: "Từ khóa", Required: true, Location: domain.ToolParamQuery}})
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &rows); err != nil {
		t.Fatal(err)
	}
	if _, ok := rows[0]["fields"]; ok {
		t.Fatalf("không nên ghi khóa fields khi rỗng: %s", encoded)
	}
	if _, ok := rows[0]["item_type"]; ok {
		t.Fatalf("không nên ghi khóa item_type khi rỗng: %s", encoded)
	}
}
