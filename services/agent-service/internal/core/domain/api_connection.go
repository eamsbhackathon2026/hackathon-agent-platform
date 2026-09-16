package domain

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// APIConnection stores reusable endpoint and credential configuration for HTTP tools.
type APIConnection struct {
	ID                      uuid.UUID
	Slug, DisplayName       string
	BaseURL                 string
	PublicHeaders           map[string]string
	SecretHeadersCiphertext []byte
	SecretHeaderNames       []string
	CreatedAt, UpdatedAt    time.Time
}

// ValidateAPIConnection validates reusable configuration before encryption or persistence.
func ValidateAPIConnection(connection APIConnection, secretHeaders map[string]string) error {
	if !slugPattern.MatchString(connection.Slug) {
		return Invalid("slug", "Mã kết nối chỉ gồm chữ, số, gạch ngang hoặc gạch dưới và dài tối đa 64 ký tự.")
	}
	if err := catalogText("display_name", connection.DisplayName, 1, 200); err != nil {
		return err
	}
	if err := catalogText("base_url", connection.BaseURL, 1, 2048); err != nil {
		return err
	}
	parsed, err := url.Parse(connection.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Invalid("base_url", "Địa chỉ kết nối phải là URL HTTP hoặc HTTPS tuyệt đối, không chứa query, fragment hoặc thông tin đăng nhập.")
	}
	return validateToolHeaders(connection.PublicHeaders, secretHeaders)
}

// JoinAPIConnectionURL resolves a validated connection base and operation path without
// allowing the operation to replace the connection scheme or host.
func JoinAPIConnectionURL(baseURL, pathTemplate string) (string, error) {
	if err := validateConnectionPath(pathTemplate); err != nil {
		return "", err
	}
	return strings.TrimRight(baseURL, "/") + pathTemplate, nil
}

func validateConnectionPath(value string) error {
	decoded := value
	for {
		next, err := url.PathUnescape(decoded)
		if err != nil {
			return Invalid("url_template", "Đường dẫn công cụ phải bắt đầu bằng một dấu / và không được chứa host, query hoặc fragment.")
		}
		if next == decoded {
			break
		}
		decoded = next
	}
	if len(value) == 0 || utf8.RuneCountInString(value) > 2048 || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.HasPrefix(decoded, "//") || strings.ContainsAny(decoded, "?#\\") {
		return Invalid("url_template", "Đường dẫn công cụ phải bắt đầu bằng một dấu / và không được chứa host, query hoặc fragment.")
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return Invalid("url_template", "Đường dẫn công cụ không được chứa đoạn tương đối '.' hoặc '..'.")
		}
	}
	return nil
}
