package domain

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ValidateProvider validates the merged persisted configuration, without secrets.
func ValidateProvider(p Provider) error {
	if err := catalogText("name", p.Name, 1, 200); err != nil {
		return err
	}
	if p.Kind != ProviderGemini && p.Kind != ProviderGreenNode && p.Kind != ProviderOpenAICompatible {
		return Invalid("kind", "Loại kết nối AI không hợp lệ.")
	}
	if p.Kind == ProviderOpenAICompatible && (p.BaseURL == nil || strings.TrimSpace(*p.BaseURL) == "") {
		return Invalid("base_url", "Nhập địa chỉ kết nối AI.")
	}
	if p.Kind == ProviderGreenNode && p.BaseURL != nil {
		return Invalid("base_url", "GreenNode sử dụng địa chỉ kết nối được quản lý sẵn.")
	}
	if p.BaseURL != nil {
		if err := catalogText("base_url", *p.BaseURL, 1, 2048); err != nil {
			return err
		}
	}
	if p.DefaultModel != nil {
		return catalogText("default_model", *p.DefaultModel, 0, 200)
	}
	return nil
}

// ValidateProviderKey checks a supplied replacement; an explicit null is handled by the service.
func ValidateProviderKey(key string) error { return catalogText("api_key", key, 1, 8192) }

// ValidateAgent checks a complete assistant configuration after applying defaults or patches.
func ValidateAgent(a Agent) error {
	for _, v := range []struct {
		field, value string
		min, max     int
	}{
		{"name", a.Name, 1, 200}, {"description", a.Description, 0, 4000}, {"model", a.Model, 1, 200}, {"system_prompt", a.SystemPrompt, 0, 100000},
	} {
		if err := catalogText(v.field, v.value, v.min, v.max); err != nil {
			return err
		}
	}
	if a.ProviderID == uuid.Nil {
		return Invalid("provider_id", "Chọn kết nối AI.")
	}
	if a.Temperature != nil && (math.IsNaN(*a.Temperature) || math.IsInf(*a.Temperature, 0) || *a.Temperature < 0 || *a.Temperature > 2) {
		return Invalid("temperature", "Độ sáng tạo phải từ 0 đến 2.")
	}
	if a.MaxOutputTokens != nil && *a.MaxOutputTokens < 1 {
		return Invalid("max_output_tokens", "Giới hạn đầu ra phải lớn hơn 0.")
	}
	if a.ContextWindowTokens < MinContextWindowTokens || a.ContextWindowTokens > MaxContextWindowTokens {
		return Invalid("context_window_tokens", "Dung lượng hội thoại phải từ 8.192 đến 2.000.000 token.")
	}
	if EffectiveMaxOutputTokens(a) > MaxOutputTokensForContext(a.ContextWindowTokens) {
		return Invalid("max_output_tokens", "Giới hạn đầu ra phải chừa đủ dung lượng cho instructions, tools và tin nhắn.")
	}
	if a.MaxIterations < 1 || a.MaxIterations > 25 {
		return Invalid("max_iterations", "Số bước tối đa phải từ 1 đến 25.")
	}
	if a.TimeoutSeconds < 10 || a.TimeoutSeconds > 600 {
		return Invalid("timeout_seconds", "Thời gian chờ phải từ 10 đến 600 giây.")
	}
	return nil
}

func catalogText(field, value string, minimum, maximum int) error {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum || (minimum > 0 && strings.TrimSpace(value) == "") {
		return Invalid(field, "Giá trị trống hoặc vượt giới hạn cho phép.")
	}
	return nil
}
