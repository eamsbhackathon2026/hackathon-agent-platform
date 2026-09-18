package domain_test

import (
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

// The label a customer reads mid-answer comes from configuration, so the order in
// which candidates win is the whole contract here.
func TestToolStepLabelPrefersTheLabelWrittenForThatMoment(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		candidates []string
		want       string
	}{
		{"nhãn bước thắng", []string{"Đang đọc chi tiêu", "Tổng hợp chi tiêu", "http_get_monthly_summary"}, "Đang đọc chi tiêu"},
		{"thiếu nhãn bước thì tới tên hiển thị", []string{"  ", "Tổng hợp chi tiêu", "http_get_monthly_summary"}, "Tổng hợp chi tiêu"},
		{"không nhãn nào thì tên công cụ", []string{"", "", "http_get_monthly_summary"}, "http_get_monthly_summary"},
		{"không có ứng viên nào", nil, ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := domain.ToolStepLabel(testCase.candidates...); got != testCase.want {
				t.Fatalf("nhãn=%q muốn %q", got, testCase.want)
			}
		})
	}
}

func TestValidateHTTPToolRejectsStepLabelThatBreaksTheRunningList(t *testing.T) {
	base := domain.HTTPTool{
		Slug: "weather", DisplayName: "Thời tiết", Description: "Xem thời tiết",
		Method: domain.HTTPToolGET, URLTemplate: "https://api.example.com/weather",
		TimeoutSeconds: 15,
	}
	if err := domain.ValidateHTTPTool(base, nil); err != nil {
		t.Fatalf("nhãn bước rỗng phải hợp lệ: %v", err)
	}
	tooLong := base
	tooLong.StepLabel = strings.Repeat("a", 81)
	if err := domain.ValidateHTTPTool(tooLong, nil); err == nil {
		t.Fatal("nhãn 81 ký tự được nhận")
	}
	// Giới hạn đếm theo KÝ TỰ, không theo byte: nhãn tiếng Việt nào cũng hai ba
	// byte một chữ, đếm byte thì 80 ký tự thành quá dài.
	vietnamese := base
	vietnamese.StepLabel = strings.Repeat("ạ", 80)
	if err := domain.ValidateHTTPTool(vietnamese, nil); err != nil {
		t.Fatalf("nhãn 80 ký tự tiếng Việt bị từ chối: %v", err)
	}
	vietnamese.StepLabel = strings.Repeat("ạ", 81)
	if err := domain.ValidateHTTPTool(vietnamese, nil); err == nil {
		t.Fatal("nhãn 81 ký tự tiếng Việt được nhận")
	}
	// Khoảng trắng thừa hai đầu không tính vào giới hạn vì không ai nhìn thấy nó.
	padded := base
	padded.StepLabel = "  " + strings.Repeat("a", 80) + "  "
	if err := domain.ValidateHTTPTool(padded, nil); err != nil {
		t.Fatalf("nhãn 80 ký tự kèm khoảng trắng bị từ chối: %v", err)
	}
	// Ký tự vô hình không tới từ bàn phím người vận hành mà tới từ bundle nhập vào.
	for name, label := range map[string]string{
		"xuống dòng":        "Đang đọc\nchi tiêu",
		"tab":               "Đang đọc\tchi tiêu",
		"tách dòng unicode": "Đang đọc\u2028chi tiêu",
		"đảo chiều chữ":     "Đang đọc\u202echi tiêu",
	} {
		broken := base
		broken.StepLabel = label
		if err := domain.ValidateHTTPTool(broken, nil); err == nil {
			t.Fatalf("nhãn chứa %s được nhận", name)
		}
	}
}
