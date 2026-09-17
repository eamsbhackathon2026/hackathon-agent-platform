package domain

import (
	"strings"
	"testing"
)

func intPtr(value int) *int { return &value }

// The whole point of the split is that a rejected request and a broken target lead the
// model to opposite next moves, so assert on that instruction rather than on wording.
func TestToolErrorGuidanceTellsRejectedRequestApartFromBrokenTarget(t *testing.T) {
	rejected := ToolErrorGuidance(intPtr(404))
	if !strings.Contains(rejected, "404") || !strings.Contains(rejected, "tham số") {
		t.Fatalf("4xx phải chỉ mô hình sửa tham số: %q", rejected)
	}
	if strings.Contains(rejected, "không khắc phục được") {
		t.Fatalf("4xx không được khuyên bỏ cuộc: %q", rejected)
	}
	broken := ToolErrorGuidance(intPtr(500))
	if !strings.Contains(broken, "500") || !strings.Contains(broken, "không khắc phục được") {
		t.Fatalf("5xx phải nói gọi lại y nguyên là vô ích: %q", broken)
	}
	if !strings.Contains(broken, "không tự suy đoán") {
		t.Fatalf("5xx phải cấm mô hình bịa kết quả: %q", broken)
	}
}

func TestToolErrorGuidanceTreatsBusyTargetAsWorthRetrying(t *testing.T) {
	for _, status := range []int{408, 429} {
		guidance := ToolErrorGuidance(intPtr(status))
		if !strings.Contains(guidance, "thử lại") {
			t.Fatalf("HTTP %d nên được coi là thử lại được: %q", status, guidance)
		}
	}
}

// A transport failure never reached the target, so there is no status to classify and
// the wording must stay as it was for callers that already rely on it.
func TestToolErrorGuidanceFallsBackWithoutStatus(t *testing.T) {
	if guidance := ToolErrorGuidance(nil); guidance != toolErrorWithoutStatus {
		t.Fatalf("thiếu status phải dùng câu chung: %q", guidance)
	}
	if summary := ToolErrorSummary(nil); summary != "Công cụ trả về lỗi." {
		t.Fatalf("tóm tắt span đổi khi chưa có status: %q", summary)
	}
}

// Operators reading the timeline should not be handed instructions written for the model.
func TestToolErrorSummaryNamesStatusWithoutInstructingTheModel(t *testing.T) {
	summary := ToolErrorSummary(intPtr(500))
	if !strings.Contains(summary, "500") {
		t.Fatalf("tóm tắt phải nêu mã lỗi: %q", summary)
	}
	if strings.Contains(summary, "gọi lại") || strings.Contains(summary, "tham số") {
		t.Fatalf("tóm tắt không được chứa chỉ dẫn dành cho mô hình: %q", summary)
	}
}

// The body often names the missing record, which is what lets the model correct itself.
func TestDescribeToolErrorKeepsTargetResponse(t *testing.T) {
	described := DescribeToolError(intPtr(404), `{"detail":"customer 999999 không tồn tại"}`)
	if !strings.Contains(described, "customer 999999 không tồn tại") {
		t.Fatalf("mất phản hồi của hệ thống đích: %q", described)
	}
	if !strings.HasPrefix(described, ToolErrorGuidance(intPtr(404))) {
		t.Fatalf("chỉ dẫn phải đứng trước phản hồi: %q", described)
	}
	if bodyless := DescribeToolError(intPtr(500), ""); bodyless != ToolErrorGuidance(intPtr(500)) {
		t.Fatalf("không có body thì không được thêm dòng thừa: %q", bodyless)
	}
}
