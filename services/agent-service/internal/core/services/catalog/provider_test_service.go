package catalog

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"context"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) client(ctx context.Context, p domain.Provider) (outbound.LLMClient, error) {
	if len(p.APIKeyCiphertext) == 0 {
		return nil, domain.ErrProviderNotConfigured
	}
	key, err := s.Cipher.Decrypt(p.APIKeyCiphertext, []byte("provider:"+p.ID.String()))
	if err != nil {
		return nil, domain.ErrProviderNotConfigured
	}
	conn := domain.ProviderConnection{Kind: p.Kind, APIKey: string(key)}
	if p.BaseURL != nil {
		conn.BaseURL = *p.BaseURL
		if s.URLs.ValidateURL(conn.BaseURL) != nil {
			return nil, domain.ErrEgressDenied
		}
	}
	return s.Factory.New(ctx, conn)
}

// TestProvider performs remote work outside transactions and applies its result with revision CAS.
func (s *Service) TestProvider(ctx context.Context, principal domain.Principal, id uuid.UUID) (inbound.ProviderTestResult, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return inbound.ProviderTestResult{}, err
	}
	p, err := s.Providers.GetProvider(ctx, id)
	if err != nil {
		return inbound.ProviderTestResult{}, err
	}
	remote, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client, err := s.client(remote, p)
	if err == nil {
		_, err = client.ListModels(remote)
		if errors.Is(err, domain.ErrModelsUnsupported) && p.DefaultModel != nil && strings.TrimSpace(*p.DefaultModel) != "" {
			one := 1
			_, err = client.Stream(remote, domain.LLMRequest{Model: *p.DefaultModel, Messages: []domain.ChatMessage{{Role: "user", Text: "Hi"}}, MaxOutputTokens: &one}, nil)
		}
	}
	result := inbound.ProviderTestResult{OK: err == nil, Message: "Kết nối AI hoạt động."}
	status := domain.ConnectionOK
	if err != nil {
		result.Failure = providerFailure(err)
		result.Message = result.Failure.Message
		status = domain.ConnectionFailing
	}
	applied, saveErr := s.Providers.RecordProviderCheck(ctx, id, p.Revision, status, result.Failure, s.Clock.Now())
	if saveErr != nil {
		return inbound.ProviderTestResult{}, saveErr
	}
	if !applied {
		return inbound.ProviderTestResult{}, &domain.Error{Kind: domain.ErrConflict, Detail: "Kết nối đã thay đổi. Hãy kiểm tra lại."}
	}
	return result, nil
}

// ListProviderModels sorts and removes duplicate IDs before cursor pagination.
func (s *Service) ListProviderModels(ctx context.Context, principal domain.Principal, id uuid.UUID, req inbound.PageRequest) (page inbound.ModelPage, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	if req.Limit < 0 || req.Limit > 100 {
		return page, domain.Invalid("limit", "Giới hạn phải từ 1 đến 100.")
	}
	if req.Limit == 0 {
		req.Limit = 25
	}
	after := ""
	if req.Cursor != "" {
		var b []byte
		b, err = base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || len(b) == 0 || !utf8.Valid(b) {
			return page, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
		}
		after = string(b)
	}
	p, err := s.Providers.GetProvider(ctx, id)
	if err != nil {
		return
	}
	remote, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client, err := s.client(remote, p)
	if err != nil {
		return page, safeProviderError(err)
	}
	models, err := client.ListModels(remote)
	if err != nil {
		return page, safeProviderError(err)
	}
	sort.Strings(models)
	page.Items = []string{}
	for _, id := range models {
		if id == "" || id <= after || (len(page.Items) > 0 && page.Items[len(page.Items)-1] == id) {
			continue
		}
		page.Items = append(page.Items, id)
	}
	if len(page.Items) > req.Limit {
		page.Items = page.Items[:req.Limit]
		cursor := base64.RawURLEncoding.EncodeToString([]byte(page.Items[len(page.Items)-1]))
		page.NextCursor = &cursor
	}
	return page, nil
}

func safeProviderError(err error) error {
	for _, known := range []error{domain.ErrProviderNotConfigured, domain.ErrProviderAuth, domain.ErrProviderUnreachable, domain.ErrModelNotFound, domain.ErrProviderRateLimited, domain.ErrProviderBadRequest, domain.ErrModelsUnsupported, domain.ErrEgressDenied} {
		if errors.Is(err, known) {
			return known
		}
	}
	return domain.ErrProviderUnreachable
}

func providerFailure(err error) *domain.ProviderFailure {
	f := domain.ProviderFailure{Code: "provider_unreachable", Message: "Không thể kết nối AI. Kiểm tra địa chỉ và thử lại."}
	switch {
	case errors.Is(err, domain.ErrProviderNotConfigured):
		f = domain.ProviderFailure{Code: "provider_not_configured", Message: "Thêm khóa bí mật để sử dụng kết nối AI."}
	case errors.Is(err, domain.ErrProviderAuth):
		f = domain.ProviderFailure{Code: "provider_auth_failed", Message: "Khóa bí mật không hợp lệ hoặc không có quyền truy cập."}
	case errors.Is(err, domain.ErrModelNotFound):
		f = domain.ProviderFailure{Code: "model_not_found", Message: "Không tìm thấy mô hình. Chọn mô hình khác."}
	case errors.Is(err, domain.ErrProviderRateLimited):
		f = domain.ProviderFailure{Code: "rate_limited", Message: "Kết nối AI đang giới hạn yêu cầu. Thử lại sau."}
	case errors.Is(err, domain.ErrProviderBadRequest), errors.Is(err, domain.ErrEgressDenied):
		f = domain.ProviderFailure{Code: "validation_failed", Message: "Kiểm tra cấu hình kết nối AI và địa chỉ được phép."}
	case errors.Is(err, domain.ErrModelsUnsupported):
		f = domain.ProviderFailure{Code: "not_implemented", Message: "Kết nối không hỗ trợ danh sách mô hình. Nhập mô hình mặc định rồi kiểm tra lại."}
	}
	return &f
}
