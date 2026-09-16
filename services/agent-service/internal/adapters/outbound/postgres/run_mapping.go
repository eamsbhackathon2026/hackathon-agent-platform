package postgres

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
)

type storedToolCall struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Arguments    json.RawMessage `json:"arguments"`
	ProviderMeta json.RawMessage `json:"provider_meta,omitempty"`
}

func idPointer(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	id := uuid.UUID(v.Bytes)
	return &id
}

func int64Value(v *int) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: int64(*v), Valid: true}
}

func intPointer(v pgtype.Int8) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func messageToolCalls(calls []domain.ToolCall) ([]byte, error) {
	stored := make([]storedToolCall, 0, len(calls))
	for _, call := range calls {
		if !json.Valid(call.Arguments) || (len(call.ProviderMeta) > 0 && !json.Valid(call.ProviderMeta)) {
			return nil, domain.ErrValidation
		}
		stored = append(stored, storedToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments, ProviderMeta: call.ProviderMeta})
	}
	return json.Marshal(stored)
}

func messageModel(v sqlcgen.Message) (domain.Message, error) {
	var stored []storedToolCall
	if err := json.Unmarshal(v.ToolCalls, &stored); err != nil {
		return domain.Message{}, fmt.Errorf("decode message tool calls: %w", err)
	}
	calls := make([]domain.ToolCall, 0, len(stored))
	for _, call := range stored {
		calls = append(calls, domain.ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments, ProviderMeta: call.ProviderMeta})
	}
	return domain.Message{ID: uuid.UUID(v.ID.Bytes), SessionID: uuid.UUID(v.SessionID.Bytes), RunID: idPointer(v.RunID), Seq: v.Seq, Role: v.Role, Content: v.Content, ToolCalls: calls, ToolCallID: stringPointer(v.ToolCallID), ToolName: stringPointer(v.ToolName), ProviderMeta: v.ProviderMeta, IsError: v.IsError, CreatedAt: v.CreatedAt.Time.UTC()}, nil
}

func sessionModel(v sqlcgen.Session) domain.Session {
	return domain.Session{ID: uuid.UUID(v.ID.Bytes), AgentID: uuid.UUID(v.AgentID.Bytes), Source: domain.RunSource(v.Source), CreatedByUserID: idPointer(v.CreatedByUserID), CreatedByAPIKeyID: idPointer(v.CreatedByApiKeyID), ExternalKey: stringPointer(v.ExternalKey), Title: v.Title, LastPromptEstimatedTokens: intPointer(v.LastPromptEstimatedTokens), LastPromptTokens: intPointer(v.LastPromptTokens), CreatedAt: v.CreatedAt.Time.UTC(), UpdatedAt: v.UpdatedAt.Time.UTC()}
}

func runModel(v sqlcgen.Run) (domain.Run, error) {
	var input domain.RunInput
	if err := json.Unmarshal(v.Input, &input); err != nil {
		return domain.Run{}, fmt.Errorf("decode run input: %w", err)
	}
	r := domain.Run{ID: uuid.UUID(v.ID.Bytes), AgentID: uuid.UUID(v.AgentID.Bytes), SessionID: uuid.UUID(v.SessionID.Bytes), Mode: domain.RunMode(v.Mode), Status: domain.RunStatus(v.Status), Source: domain.RunSource(v.Source), TriggeredByUserID: idPointer(v.TriggeredByUserID), TriggeredByAPIKeyID: idPointer(v.TriggeredByApiKeyID), Input: input, Output: stringPointer(v.Output), Iterations: int(v.Iterations), Usage: domain.TokenUsage{InputTokens: intPointer(v.InputTokens), OutputTokens: intPointer(v.OutputTokens)}, Metadata: v.Metadata, WebhookURL: stringPointer(v.WebhookUrl), CancelRequestedAt: timePointer(v.CancelRequestedAt), QueuedAt: timePointer(v.QueuedAt), StartedAt: timePointer(v.StartedAt), FinishedAt: timePointer(v.FinishedAt), CreatedAt: v.CreatedAt.Time.UTC()}
	if v.ErrorCode.Valid {
		r.Failure = &domain.RunFailure{Code: v.ErrorCode.String, Message: v.ErrorMessage.String}
	}
	return r, nil
}

func spanModel(v sqlcgen.Span) domain.Span {
	return domain.Span{ID: uuid.UUID(v.ID.Bytes), RunID: uuid.UUID(v.RunID.Bytes), ParentSpanID: idPointer(v.ParentSpanID), Kind: domain.SpanKind(v.Kind), Name: v.Name, Status: domain.SpanStatus(v.Status), Model: stringPointer(v.Model), ToolName: stringPointer(v.ToolName), Usage: domain.TokenUsage{InputTokens: intPointer(v.InputTokens), OutputTokens: intPointer(v.OutputTokens)}, StartedAt: v.StartedAt.Time.UTC(), EndedAt: v.EndedAt.Time.UTC(), DurationMS: v.DurationMs, Attributes: v.Attributes, ErrorMessage: stringPointer(v.ErrorMessage)}
}
