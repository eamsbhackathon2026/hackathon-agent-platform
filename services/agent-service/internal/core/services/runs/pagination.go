package runs

import (
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

func descendingPage(p inbound.PageRequest) (domain.PageOptions, error) {
	limit, err := pageLimit(p.Limit)
	if err != nil {
		return domain.PageOptions{}, err
	}
	options := domain.PageOptions{Limit: limit + 1}
	if p.Cursor == "" {
		return options, nil
	}
	var cursor domain.PageCursor
	if err = decodeCursor(p.Cursor, &cursor); err != nil || cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return options, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
	}
	options.Before = &cursor
	return options, nil
}

func updatedSessionPage(p inbound.PageRequest) (int, *outbound.SessionUpdatedCursor, error) {
	limit, err := pageLimit(p.Limit)
	if err != nil || p.Cursor == "" {
		return limit + 1, nil, err
	}
	var cursor outbound.SessionUpdatedCursor
	if err = decodeCursor(p.Cursor, &cursor); err != nil || cursor.ID == uuid.Nil || cursor.UpdatedAt.IsZero() {
		return 0, nil, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
	}
	return limit + 1, &cursor, nil
}

func messagePage(p inbound.PageRequest) (int, *outbound.MessageCursor, error) {
	limit, err := pageLimit(p.Limit)
	if err != nil || p.Cursor == "" {
		return limit + 1, nil, err
	}
	var cursor outbound.MessageCursor
	if err = decodeCursor(p.Cursor, &cursor); err != nil || cursor.ID == uuid.Nil || cursor.Seq < 1 {
		return 0, nil, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
	}
	return limit + 1, &cursor, nil
}

func spanPage(p inbound.PageRequest) (int, *outbound.SpanCursor, error) {
	limit, err := pageLimit(p.Limit)
	if err != nil || p.Cursor == "" {
		return limit + 1, nil, err
	}
	var cursor outbound.SpanCursor
	if err = decodeCursor(p.Cursor, &cursor); err != nil || cursor.ID == uuid.Nil || cursor.StartedAt.IsZero() {
		return 0, nil, domain.Invalid("cursor", "Vị trí trang không hợp lệ.")
	}
	return limit + 1, &cursor, nil
}

func pageLimit(limit int) (int, error) {
	if limit < 0 || limit > 100 {
		return 0, domain.Invalid("limit", "Giới hạn phải từ 1 đến 100.")
	}
	if limit == 0 {
		limit = 25
	}
	return limit, nil
}

func decodeCursor(value string, target any) error {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func encodeCursor(value any) *string {
	raw, _ := json.Marshal(value)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return &encoded
}
