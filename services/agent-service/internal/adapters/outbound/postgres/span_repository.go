package postgres

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// CreateSpan persists one completed public-safe trace step.
func (s *Store) CreateSpan(ctx context.Context, v domain.Span) error {
	attributes := v.Attributes
	if len(attributes) == 0 {
		attributes = []byte(`{}`)
	}
	return mapError(s.queries(ctx).CreateSpan(ctx, sqlcgen.CreateSpanParams{ID: dbID(v.ID), RunID: dbID(v.RunID), ParentSpanID: optionalID(v.ParentSpanID), Kind: string(v.Kind), Name: v.Name, Status: string(v.Status), Model: textValue(v.Model), ToolName: textValue(v.ToolName), InputTokens: int64Value(v.Usage.InputTokens), OutputTokens: int64Value(v.Usage.OutputTokens), StartedAt: catalogTime(v.StartedAt), EndedAt: catalogTime(v.EndedAt), DurationMs: v.DurationMS, Attributes: attributes, ErrorMessage: textValue(v.ErrorMessage)}))
}

// ListSpans returns an ascending keyset page for one execution.
func (s *Store) ListSpans(ctx context.Context, runID uuid.UUID, limit int, after *outbound.SpanCursor) ([]domain.Span, error) {
	params := sqlcgen.ListSpansParams{RunID: dbID(runID), Limit: boundedLimit(limit)}
	if after != nil {
		params.AfterTime = catalogTime(after.StartedAt)
		params.AfterID = dbID(after.ID)
	}
	rows, err := s.queries(ctx).ListSpans(ctx, params)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.Span, 0, len(rows))
	for _, row := range rows {
		result = append(result, spanModel(row))
	}
	return result, nil
}

var _ outbound.SpanRepository = (*Store)(nil)
