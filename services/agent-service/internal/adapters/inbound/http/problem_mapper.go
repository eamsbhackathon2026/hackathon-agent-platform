package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
)

func writeProblem(w http.ResponseWriter, status int, title, code string) {
	writeProblemBody(w, gen.Problem{Type: "about:blank", Status: status, Title: title, Code: gen.ProblemCode(code), Fields: []gen.FieldError{}})
}

func writeProblemBody(w http.ResponseWriter, problem gen.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}

func writeDomainError(w http.ResponseWriter, err error) {
	status, title, code := 500, "Không thể xử lý yêu cầu", "internal"
	switch {
	case errors.Is(err, domain.ErrUnauthenticated):
		status, title, code = 401, "Thông tin đăng nhập không hợp lệ hoặc đã hết hạn", "unauthenticated"
	case errors.Is(err, domain.ErrForbidden):
		status, title, code = 403, "Bạn chưa được phép thực hiện thao tác này", "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		status, title, code = 404, "Không tìm thấy dữ liệu", "not_found"
	case errors.Is(err, domain.ErrConflict):
		status, title, code = 409, "Dữ liệu đang xung đột", "conflict"
	case errors.Is(err, domain.ErrValidation):
		status, title, code = 400, "Vui lòng kiểm tra thông tin đã nhập", "validation_failed"
	case errors.Is(err, domain.ErrRateLimited):
		status, title, code = 429, "Bạn đã thử quá nhiều lần; vui lòng thử lại sau", "rate_limited"
	case errors.Is(err, domain.ErrRunInProgress):
		status, title, code = 409, "Hội thoại đang có lần xử lý chưa hoàn tất", "run_in_progress"
	case errors.Is(err, domain.ErrIdempotencyKeyReused):
		status, title, code = 422, "Khóa chống lặp đã được dùng cho yêu cầu khác", "idempotency_key_reused"
	case errors.Is(err, domain.ErrNotImplemented):
		status, title, code = 501, "Chức năng này chưa sẵn sàng", "not_implemented"
	case errors.Is(err, domain.ErrProviderNotConfigured):
		status, title, code = 400, "Hãy bổ sung thông tin kết nối AI", "provider_not_configured"
	case errors.Is(err, domain.ErrProviderAuth):
		status, title, code = 502, "Khóa của kết nối AI không hợp lệ; hãy cập nhật khóa", "provider_auth_failed"
	case errors.Is(err, domain.ErrProviderRateLimited):
		status, title, code = 429, "Kết nối AI đang giới hạn yêu cầu; hãy thử lại sau", "rate_limited"
	case errors.Is(err, domain.ErrModelNotFound):
		status, title, code = 404, "Không tìm thấy mô hình; hãy chọn hoặc nhập lại tên", "model_not_found"
	case errors.Is(err, domain.ErrProviderBadRequest):
		status, title, code = 400, "Kết nối AI không chấp nhận yêu cầu; hãy kiểm tra cấu hình", "validation_failed"
	case errors.Is(err, domain.ErrModelsUnsupported):
		status, title, code = 501, "Kết nối này không cung cấp danh sách mô hình; hãy nhập tên mô hình", "not_implemented"
	case errors.Is(err, domain.ErrProviderUnreachable), errors.Is(err, domain.ErrEgressDenied):
		status, title, code = 502, "Không thể truy cập kết nối AI; hãy kiểm tra địa chỉ và danh sách cho phép của máy chủ", "provider_unreachable"
	}
	problem := gen.Problem{Type: "about:blank", Status: status, Title: title, Code: gen.ProblemCode(code), Fields: []gen.FieldError{}}
	var detail *domain.Error
	if status != 500 && errors.As(err, &detail) {
		problem.Detail = detail.Detail
		problem.RunId = detail.RunID
		for _, field := range detail.Fields {
			problem.Fields = append(problem.Fields, gen.FieldError{Field: field.Field, Message: field.Message})
		}
		if len(detail.RelatedAgents) > 0 {
			related := make([]gen.ResourceReference, len(detail.RelatedAgents))
			for i, a := range detail.RelatedAgents {
				related[i] = gen.ResourceReference{Id: a.ID, Name: a.Name}
			}
			problem.RelatedAgents = &related
		}
	}
	writeProblemBody(w, problem)
}
