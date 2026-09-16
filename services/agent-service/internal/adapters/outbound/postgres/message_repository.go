package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// AppendMessage atomically allocates the next sequence number and stores the entry.
func (s *Store) AppendMessage(ctx context.Context, v domain.Message) (domain.Message, error) {
	calls, err := messageToolCalls(v.ToolCalls)
	if err != nil {
		return domain.Message{}, err
	}
	if len(v.ProviderMeta) > 0 && !jsonValid(v.ProviderMeta) {
		return domain.Message{}, domain.ErrValidation
	}
	row, err := s.queries(ctx).AppendMessage(ctx, sqlcgen.AppendMessageParams{ID: dbID(v.ID), SessionID: dbID(v.SessionID), RunID: optionalID(v.RunID), Role: v.Role, Content: v.Content, ToolCalls: calls, ToolCallID: textValue(v.ToolCallID), ToolName: textValue(v.ToolName), ProviderMeta: v.ProviderMeta, IsError: v.IsError, CreatedAt: catalogTime(v.CreatedAt)})
	if err != nil {
		return domain.Message{}, mapError(err)
	}
	return messageModel(row)
}

// ListRecentMessages returns the newest bounded history in ascending order.
func (s *Store) ListRecentMessages(ctx context.Context, sessionID uuid.UUID, limit int) ([]domain.Message, error) {
	rows, err := s.queries(ctx).ListRecentMessages(ctx, sqlcgen.ListRecentMessagesParams{SessionID: dbID(sessionID), Limit: boundedLimit(limit)})
	return mapMessages(rows, err)
}

// ListMessages returns an ascending keyset page.
func (s *Store) ListMessages(ctx context.Context, sessionID uuid.UUID, limit int, after *outbound.MessageCursor) ([]domain.Message, error) {
	params := sqlcgen.ListMessagesParams{SessionID: dbID(sessionID), Limit: boundedLimit(limit)}
	if after != nil {
		params.AfterSeq.Valid, params.AfterSeq.Int64 = true, after.Seq
		params.AfterID = dbID(after.ID)
	}
	rows, err := s.queries(ctx).ListMessages(ctx, params)
	return mapMessages(rows, err)
}

// ListRunMessages returns one ascending page for a single run in a session.
func (s *Store) ListRunMessages(ctx context.Context, sessionID, runID uuid.UUID, limit int, after *outbound.MessageCursor) ([]domain.Message, error) {
	params := sqlcgen.ListRunMessagesParams{SessionID: dbID(sessionID), RunID: dbID(runID), Limit: boundedLimit(limit)}
	if after != nil {
		params.AfterSeq.Valid, params.AfterSeq.Int64 = true, after.Seq
		params.AfterID = dbID(after.ID)
	}
	rows, err := s.queries(ctx).ListRunMessages(ctx, params)
	return mapMessages(rows, err)
}

// ListMessagesAfterSeq returns an ascending internal page after a durable checkpoint.
func (s *Store) ListMessagesAfterSeq(ctx context.Context, sessionID uuid.UUID, afterSeq int64, limit int) ([]domain.Message, error) {
	if afterSeq < 0 {
		return nil, domain.ErrValidation
	}
	rows, err := s.queries(ctx).ListMessagesAfterSeq(ctx, sqlcgen.ListMessagesAfterSeqParams{SessionID: dbID(sessionID), AfterSeq: afterSeq, Limit: boundedLimit(limit)})
	return mapMessages(rows, err)
}

func mapMessages(rows []sqlcgen.Message, err error) ([]domain.Message, error) {
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]domain.Message, 0, len(rows))
	for _, row := range rows {
		message, decodeErr := messageModel(row)
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = append(result, message)
	}
	return result, nil
}

func boundedLimit(limit int) int32 {
	if limit < 1 {
		return 50
	}
	if limit > 101 {
		return 101
	}
	return int32(limit)
}

func jsonValid(value []byte) bool {
	var target any
	return json.Unmarshal(value, &target) == nil
}

var _ outbound.MessageRepository = (*Store)(nil)
