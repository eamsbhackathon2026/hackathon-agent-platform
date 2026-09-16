package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SpanKind identifies the measured execution step.
type SpanKind string

const (
	// SpanRun measures the complete execution.
	SpanRun SpanKind = "run"
	// SpanLLMCall measures one provider generation.
	SpanLLMCall SpanKind = "llm_call"
	// SpanToolCall measures one tool invocation.
	SpanToolCall SpanKind = "tool_call"
)

// SpanStatus records whether a measured step completed successfully.
type SpanStatus string

const (
	// SpanOK records successful completion.
	SpanOK SpanStatus = "ok"
	// SpanError records a failed or cancelled step.
	SpanError SpanStatus = "error"
)

// Span is a compact, public-safe trace record.
type Span struct {
	ID, RunID          uuid.UUID
	ParentSpanID       *uuid.UUID
	Kind               SpanKind
	Name               string
	Status             SpanStatus
	Model, ToolName    *string
	Usage              TokenUsage
	StartedAt, EndedAt time.Time
	DurationMS         int64
	Attributes         json.RawMessage
	ErrorMessage       *string
}
