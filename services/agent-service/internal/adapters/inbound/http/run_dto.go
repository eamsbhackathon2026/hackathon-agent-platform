package http

import (
	"encoding/json"

	"github.com/oapi-codegen/nullable"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
)

func runDTO(run domain.Run) (gen.Run, error) {
	metadata := map[string]any{}
	if len(run.Metadata) > 0 {
		if err := json.Unmarshal(run.Metadata, &metadata); err != nil {
			return gen.Run{}, err
		}
	}
	var failure *failureDTO
	if run.Failure != nil {
		failure = &failureDTO{Code: gen.ProblemCode(run.Failure.Code), Message: run.Failure.Message}
	}
	toolResults, err := toolResultsDTO(run.ToolResults)
	if err != nil {
		return gen.Run{}, err
	}
	return gen.Run{Id: run.ID, AgentId: run.AgentID, SessionId: run.SessionID, Mode: gen.RunMode(run.Mode), Status: gen.RunStatus(run.Status), Source: gen.RunSource(run.Source), TriggeredByUserId: nullableValue(run.TriggeredByUserID), TriggeredByApiKeyId: nullableValue(run.TriggeredByAPIKeyID), Input: gen.RunInput{Message: run.Input.Message}, Output: nullableValue(run.Output), Error: nullableValue(failure), Iterations: run.Iterations, Usage: usageDTO(run.Usage), Metadata: metadata, CancelRequestedAt: nullableValue(run.CancelRequestedAt), QueuedAt: nullableValue(run.QueuedAt), StartedAt: nullableValue(run.StartedAt), FinishedAt: nullableValue(run.FinishedAt), CreatedAt: run.CreatedAt, ToolResults: toolResults}, nil
}

// toolResultsDTO keeps nil (not loaded) distinct from an empty list (no tool calls).
func toolResultsDTO(results []domain.RunToolResult) (nullable.Nullable[[]gen.RunToolResult], error) {
	var out nullable.Nullable[[]gen.RunToolResult]
	if results == nil {
		out.SetNull()
		return out, nil
	}
	items := make([]gen.RunToolResult, 0, len(results))
	for _, item := range results {
		arguments := map[string]any{}
		if len(item.Arguments) > 0 {
			if err := json.Unmarshal(item.Arguments, &arguments); err != nil {
				return out, err
			}
		}
		var result any
		if len(item.Result) > 0 {
			if err := json.Unmarshal(item.Result, &result); err != nil {
				return out, err
			}
		}
		items = append(items, gen.RunToolResult{CallId: item.CallID, ToolName: item.ToolName, Arguments: arguments, Result: result, IsError: item.IsError})
	}
	out.Set(items)
	return out, nil
}

func usageDTO(usage domain.TokenUsage) gen.RunUsage {
	var input, output *int64
	if usage.InputTokens != nil {
		value := int64(*usage.InputTokens)
		input = &value
	}
	if usage.OutputTokens != nil {
		value := int64(*usage.OutputTokens)
		output = &value
	}
	return gen.RunUsage{InputTokens: nullableValue(input), OutputTokens: nullableValue(output)}
}

func sessionDTO(session domain.Session) gen.Session {
	return gen.Session{Id: session.ID, AgentId: session.AgentID, Source: gen.RunSource(session.Source), CreatedByUserId: nullableValue(session.CreatedByUserID), CreatedByApiKeyId: nullableValue(session.CreatedByAPIKeyID), ExternalKey: nullableValue(session.ExternalKey), Title: session.Title, CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt}
}

func sessionSummaryDTO(summary domain.SessionSummary) gen.SessionSummary {
	dto := gen.SessionSummary{TurnCount: summary.TurnCount, FailedTurnCount: summary.FailedTurnCount, LatestRunId: nullableValue(summary.LatestRunID), Usage: usageDTO(summary.Usage), ProcessingMs: summary.ProcessingMS, FirstMessage: nullableValue(summary.FirstMessage), LastMessage: nullableValue(summary.LastMessage)}
	dto.LatestRunStatus.SetNull()
	if summary.LatestRunStatus != nil {
		dto.LatestRunStatus.Set(gen.SessionSummaryLatestRunStatus(*summary.LatestRunStatus))
	}
	dto.LastMessageRole.SetNull()
	if summary.LastMessageRole != nil {
		dto.LastMessageRole.Set(gen.SessionSummaryLastMessageRole(*summary.LastMessageRole))
	}
	return dto
}

func messageDTO(message domain.Message) (gen.Message, error) {
	calls := make([]gen.ToolCall, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		arguments := map[string]any{}
		if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
			return gen.Message{}, err
		}
		calls = append(calls, gen.ToolCall{Id: call.ID, Name: call.Name, Arguments: arguments})
	}
	return gen.Message{Id: message.ID, SessionId: message.SessionID, RunId: nullableValue(message.RunID), Seq: message.Seq, Role: gen.MessageRole(message.Role), Content: message.Content, ToolCalls: calls, ToolCallId: nullableValue(message.ToolCallID), ToolName: nullableValue(message.ToolName), IsError: message.IsError, CreatedAt: message.CreatedAt}, nil
}

func spanDTO(span domain.Span) (gen.Span, error) {
	attributes := map[string]any{}
	if len(span.Attributes) > 0 {
		if err := json.Unmarshal(span.Attributes, &attributes); err != nil {
			return gen.Span{}, err
		}
	}
	return gen.Span{Id: span.ID, RunId: span.RunID, ParentSpanId: nullableValue(span.ParentSpanID), Kind: gen.SpanKind(span.Kind), Name: span.Name, Status: gen.SpanStatus(span.Status), Model: nullableValue(span.Model), ToolName: nullableValue(span.ToolName), Usage: usageDTO(span.Usage), StartedAt: span.StartedAt, EndedAt: span.EndedAt, DurationMs: span.DurationMS, Attributes: attributes, ErrorMessage: nullableValue(span.ErrorMessage)}, nil
}

func eventDTO(event domain.RunEvent) (gen.RunEvent, error) {
	var result gen.RunEvent
	switch event.Type {
	case domain.EventRunStarted:
		return result, result.FromRunStartedEvent(gen.RunStartedEvent{Type: gen.RunStarted, RunId: event.RunID, SessionId: event.SessionID})
	case domain.EventMessageDelta:
		return result, result.FromMessageDeltaEvent(gen.MessageDeltaEvent{Type: gen.MessageDelta, Text: event.Text})
	case domain.EventToolStarted:
		return result, result.FromToolStartedEvent(gen.ToolStartedEvent{Type: gen.ToolStarted, CallId: event.CallID, ToolName: event.ToolName, DisplayName: event.DisplayName})
	case domain.EventToolFinished:
		return result, result.FromToolFinishedEvent(gen.ToolFinishedEvent{Type: gen.ToolFinished, CallId: event.CallID, Ok: event.OK, DurationMs: event.DurationMS})
	case domain.EventMessageCompleted:
		message, err := messageDTO(*event.Message)
		if err != nil {
			return result, err
		}
		return result, result.FromMessageCompletedEvent(gen.MessageCompletedEvent{Type: gen.MessageCompleted, Message: message})
	case domain.EventRunCompleted:
		run, err := runDTO(*event.Run)
		if err != nil {
			return result, err
		}
		return result, result.FromRunCompletedEvent(gen.RunCompletedEvent{Type: gen.RunCompletedEventTypeRunCompleted, Run: run})
	case domain.EventRunFailed:
		run, err := runDTO(*event.Run)
		if err != nil {
			return result, err
		}
		failure := gen.RunError{Code: gen.ProblemCode(event.Failure.Code), Message: event.Failure.Message}
		return result, result.FromRunFailedEvent(gen.RunFailedEvent{Type: gen.RunFailedEventTypeRunFailed, Run: run, Error: failure})
	default:
		return result, domain.ErrValidation
	}
}
