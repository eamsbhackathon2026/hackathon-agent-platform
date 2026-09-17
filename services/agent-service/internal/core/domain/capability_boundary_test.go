package domain

import (
	"strings"
	"testing"
)

// The anti-fraud assistant that prompted this: every bound tool assesses or records, none
// moves money, yet its prompt is written in the vocabulary of transfers.
func antiFraudSpecs() []ToolSpec {
	return []ToolSpec{
		{Name: "http_precheck_transfer", Description: "Chấm điểm rủi ro một lệnh chuyển tiền TRƯỚC khi thực hiện."},
		{Name: "http_take_action", Description: "Chốt hành động cuối: cancel, hold, contact hoặc continue."},
		{Name: "http_create_case", Description: "Mở case theo dõi."},
	}
}

func TestAppendCapabilityBoundaryNamesEveryToolAndForbidsPromisingMore(t *testing.T) {
	prompt := AppendCapabilityBoundary("Bạn là trợ lý chống lừa đảo chuyển tiền.", antiFraudSpecs())
	if !strings.Contains(prompt, "Bạn là trợ lý chống lừa đảo chuyển tiền.") {
		t.Fatal("prompt của người dựng bị mất")
	}
	for _, spec := range antiFraudSpecs() {
		if !strings.Contains(prompt, spec.Name) {
			t.Fatalf("thiếu công cụ %q trong danh sách", spec.Name)
		}
	}
	if !strings.Contains(prompt, "only actions you can perform") {
		t.Fatal("không nêu đây là toàn bộ năng lực")
	}
	if !strings.Contains(prompt, "Never promise") {
		t.Fatal("thiếu luật cấm hứa việc ngoài danh sách")
	}
	// Fixing a leak of internal concepts must not create another one.
	if !strings.Contains(prompt, "never mention tool names") {
		t.Fatal("thiếu luật cấm nói tên công cụ với khách")
	}
	// The boundary has to come last so it reads as the final word over the prompt above it.
	if strings.Index(prompt, "## Capabilities") < strings.Index(prompt, "trợ lý chống lừa đảo") {
		t.Fatal("mục năng lực phải đứng sau chỉ dẫn của người dựng")
	}
}

// An assistant with nothing bound is the sharpest version of the same failure: it can only
// talk, so any promise of action is false.
func TestAppendCapabilityBoundaryHandlesAgentWithNoTools(t *testing.T) {
	prompt := AppendCapabilityBoundary("Bạn là trợ lý.", nil)
	if !strings.Contains(prompt, "You have no tools") || !strings.Contains(prompt, "cannot do it") {
		t.Fatalf("không nói rõ trợ lý không có công cụ: %q", prompt)
	}
	if strings.Contains(prompt, "only actions you can perform") {
		t.Fatal("không được liệt kê danh sách rỗng")
	}
}

func TestAppendCapabilityBoundaryStandsAloneWhenPromptIsEmpty(t *testing.T) {
	for _, prompt := range []string{"", "   \n\t "} {
		result := AppendCapabilityBoundary(prompt, antiFraudSpecs())
		if !strings.HasPrefix(result, "## Capabilities") {
			t.Fatalf("prompt rỗng không được để lại khoảng trắng thừa: %q", result)
		}
	}
}

// Descriptions come from external manifests: multi-line text would break the
// one-tool-per-line shape the model reads the inventory from.
func TestAppendCapabilityBoundaryKeepsOneToolPerLine(t *testing.T) {
	prompt := AppendCapabilityBoundary("", []ToolSpec{
		{Name: "http_a", Description: "Dòng một\nDòng hai\n\nDòng ba"},
		{Name: "http_b", Description: ""},
	})
	if strings.Contains(prompt, "Dòng một\nDòng hai") {
		t.Fatalf("mô tả nhiều dòng không được giữ nguyên xuống dòng: %q", prompt)
	}
	if !strings.Contains(prompt, "- http_a: Dòng một Dòng hai Dòng ba\n") {
		t.Fatalf("mô tả nhiều dòng phải gộp thành một dòng: %q", prompt)
	}
	// A tool with no description still has to appear; the name alone answers "can I do this".
	if !strings.Contains(prompt, "- http_b\n") {
		t.Fatalf("công cụ không mô tả vẫn phải được liệt kê: %q", prompt)
	}
}

// Nothing limits how many tools an agent may be bound to, so the section must bound itself.
func TestAppendCapabilityBoundaryFallsBackToNamesWhenSectionWouldBeHuge(t *testing.T) {
	specs := make([]ToolSpec, 200)
	for i := range specs {
		specs[i] = ToolSpec{Name: "http_tool_" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Description: strings.Repeat("mô tả rất dài ", 40)}
	}
	prompt := AppendCapabilityBoundary("", specs)
	if strings.Contains(prompt, "mô tả rất dài") {
		t.Fatal("vượt ngưỡng thì phải bỏ mô tả, chỉ giữ tên")
	}
	for _, spec := range specs {
		if !strings.Contains(prompt, spec.Name) {
			t.Fatalf("bỏ mô tả nhưng không được bỏ sót công cụ %q", spec.Name)
		}
	}
}

func TestAppendCapabilityBoundaryTrimsLongDescription(t *testing.T) {
	long := strings.Repeat("dài ", 200)
	prompt := AppendCapabilityBoundary("", []ToolSpec{{Name: "http_a", Description: long}})
	if !strings.Contains(prompt, "…") {
		t.Fatal("mô tả dài phải bị cắt và đánh dấu")
	}
	if len(prompt) > MaxCapabilitySectionBytes {
		t.Fatalf("mục vượt ngân sách: %d byte", len(prompt))
	}
}

// Real manifest descriptions lead with what the tool does, then spend far more room on
// parameter and return detail that the tool schema already carries in the same request.
func TestAppendCapabilityBoundaryKeepsOnlyTheFirstSentence(t *testing.T) {
	prompt := AppendCapabilityBoundary("", []ToolSpec{{
		Name:        "http_precheck_transfer",
		Description: "Chấm điểm rủi ro một lệnh chuyển tiền TRƯỚC khi thực hiện. Tự lấy baseline hành vi, lịch sử người nhận và sự kiện 60 phút gần nhất. Trả về: decision_id, score, level.",
	}})
	if !strings.Contains(prompt, "- http_precheck_transfer: Chấm điểm rủi ro một lệnh chuyển tiền TRƯỚC khi thực hiện.\n") {
		t.Fatalf("phải giữ đúng câu đầu: %q", prompt)
	}
	if strings.Contains(prompt, "baseline hành vi") || strings.Contains(prompt, "decision_id") {
		t.Fatalf("chi tiết sau câu đầu phải bị bỏ: %q", prompt)
	}
}

// A description that is one long sentence still has to be bounded.
func TestAppendCapabilityBoundaryTrimsSingleSentenceDescription(t *testing.T) {
	prompt := AppendCapabilityBoundary("", []ToolSpec{{Name: "http_a", Description: strings.Repeat("rất dài ", 60)}})
	if !strings.Contains(prompt, "…") {
		t.Fatalf("câu dài không xuống dòng phải bị cắt: %q", prompt)
	}
}
