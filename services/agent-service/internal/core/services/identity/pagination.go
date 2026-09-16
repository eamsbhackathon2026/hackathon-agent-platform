package identity

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
)

func pageOptions(p inbound.PageRequest) (domain.PageOptions, error) {
	if p.Limit < 0 || p.Limit > 100 {
		return domain.PageOptions{}, domain.Invalid("limit", "Giới hạn phải từ 1 đến 100.")
	}
	if p.Limit == 0 {
		p.Limit = 25
	}
	options := domain.PageOptions{Limit: p.Limit + 1}
	if p.Cursor != "" {
		var c domain.PageCursor
		b, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		if err != nil {
			return options, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
		}
		if err = json.Unmarshal(b, &c); err != nil || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
			return options, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
		}
		options.Before = &c
	}
	return options, nil
}
func encodeCursor(c domain.PageCursor) *string {
	b, _ := json.Marshal(c)
	s := base64.RawURLEncoding.EncodeToString(b)
	return &s
}
