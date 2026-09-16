package skills

import (
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

func pageOptions(request inbound.PageRequest) (domain.PageOptions, error) {
	if request.Limit < 0 || request.Limit > 100 {
		return domain.PageOptions{}, domain.Invalid("limit", "Giới hạn phải từ 1 đến 100.")
	}
	if request.Limit == 0 {
		request.Limit = 25
	}
	options := domain.PageOptions{Limit: request.Limit + 1}
	if request.Cursor == "" {
		return options, nil
	}
	encoded, err := base64.RawURLEncoding.DecodeString(request.Cursor)
	var cursor domain.PageCursor
	if err != nil || json.Unmarshal(encoded, &cursor) != nil || cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return options, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
	}
	options.Before = &cursor
	return options, nil
}

func encodeCursor(cursor domain.PageCursor) *string {
	value, _ := json.Marshal(cursor)
	encoded := base64.RawURLEncoding.EncodeToString(value)
	return &encoded
}
