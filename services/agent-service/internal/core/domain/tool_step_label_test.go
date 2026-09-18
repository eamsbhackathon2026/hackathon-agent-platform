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
		{"step label wins", []string{"Đang đọc chi tiêu", "Tổng hợp chi tiêu", "http_get_monthly_summary"}, "Đang đọc chi tiêu"},
		{"no step label falls back to the display name", []string{"  ", "Tổng hợp chi tiêu", "http_get_monthly_summary"}, "Tổng hợp chi tiêu"},
		{"no label at all falls back to the tool name", []string{"", "", "http_get_monthly_summary"}, "http_get_monthly_summary"},
		{"no candidates", nil, ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := domain.ToolStepLabel(testCase.candidates...); got != testCase.want {
				t.Fatalf("label=%q want %q", got, testCase.want)
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
		t.Fatalf("an empty step label must be valid: %v", err)
	}
	tooLong := base
	tooLong.StepLabel = strings.Repeat("a", 81)
	if err := domain.ValidateHTTPTool(tooLong, nil); err == nil {
		t.Fatal("accepted a label of 81 characters")
	}
	// The limit counts CHARACTERS, not bytes: Vietnamese spends two or three bytes
	// on most letters, so counting bytes would reject a label of 80 characters.
	vietnamese := base
	vietnamese.StepLabel = strings.Repeat("ạ", 80)
	if err := domain.ValidateHTTPTool(vietnamese, nil); err != nil {
		t.Fatalf("rejected 80 Vietnamese characters: %v", err)
	}
	vietnamese.StepLabel = strings.Repeat("ạ", 81)
	if err := domain.ValidateHTTPTool(vietnamese, nil); err == nil {
		t.Fatal("accepted 81 Vietnamese characters")
	}
	// Padding does not count towards the limit because nobody sees it.
	padded := base
	padded.StepLabel = "  " + strings.Repeat("a", 80) + "  "
	if err := domain.ValidateHTTPTool(padded, nil); err != nil {
		t.Fatalf("rejected 80 characters plus padding: %v", err)
	}
	// Invisible runes arrive from an imported bundle, not from an operator typing.
	for name, label := range map[string]string{
		"newline":        "Đang đọc\nchi tiêu",
		"tab":            "Đang đọc\tchi tiêu",
		"line separator": "Đang đọc\u2028chi tiêu",
		"bidi override":  "Đang đọc\u202echi tiêu",
	} {
		broken := base
		broken.StepLabel = label
		if err := domain.ValidateHTTPTool(broken, nil); err == nil {
			t.Fatalf("accepted a label containing a %s", name)
		}
	}
}
