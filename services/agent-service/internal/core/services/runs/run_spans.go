package runs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

func (s *Service) recordLLMSpan(ctx context.Context, state *executionState, id uuid.UUID, model string, started, ended time.Time, result domain.LLMResult, failure *domain.RunFailure) error {
	status, attributes := domain.SpanOK, map[string]any{"iteration": state.run.Iterations, "finish_reason": result.FinishReason}
	if state.promptEstimate > 0 {
		attributes["estimated_input_tokens"] = state.promptEstimate
		attributes["calibration_saved"] = state.calibrationSaved
	}
	return s.recordNamedLLMSpan(ctx, state, id, model, "llm.generate", started, ended, result, failure, status, attributes)
}

func (s *Service) recordContextCompactionSpan(ctx context.Context, state *executionState, id uuid.UUID, model string, coveredThrough int64, started, ended time.Time, result domain.LLMResult, failure *domain.RunFailure) error {
	status := domain.SpanOK
	attributes := map[string]any{"covered_through_seq": coveredThrough, "finish_reason": result.FinishReason}
	return s.recordNamedLLMSpan(ctx, state, id, model, "llm.compact_context", started, ended, result, failure, status, attributes)
}

func (s *Service) recordNamedLLMSpan(ctx context.Context, state *executionState, id uuid.UUID, model, name string, started, ended time.Time, result domain.LLMResult, failure *domain.RunFailure, status domain.SpanStatus, attributes map[string]any) error {
	var message *string
	if failure != nil {
		status, message = domain.SpanError, &failure.Message
	}
	encoded, _ := json.Marshal(attributes)
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.Spans.CreateSpan(persistCtx, domain.Span{ID: id, RunID: state.run.ID, ParentSpanID: &state.rootSpanID, Kind: domain.SpanLLMCall, Name: name, Status: status, Model: &model, Usage: result.Usage, StartedAt: started, EndedAt: ended, DurationMS: durationMS(started, ended), Attributes: encoded, ErrorMessage: message})
}

func (s *Service) recordToolSpan(ctx context.Context, state *executionState, id uuid.UUID, call domain.ToolCall, result domain.ToolResult, truncated bool, started, ended time.Time) error {
	status := domain.SpanOK
	var message *string
	if result.IsError {
		status = domain.SpanError
		safe := "Công cụ trả về lỗi."
		message = &safe
	}
	attributes, _ := json.Marshal(map[string]any{"call_id": call.ID, "result_truncated": truncated})
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.Spans.CreateSpan(persistCtx, domain.Span{ID: id, RunID: state.run.ID, ParentSpanID: &state.rootSpanID, Kind: domain.SpanToolCall, Name: "tool.execute", Status: status, ToolName: &call.Name, StartedAt: started, EndedAt: ended, DurationMS: durationMS(started, ended), Attributes: attributes, ErrorMessage: message})
}
