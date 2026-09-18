package domain

import "github.com/google/uuid"

// RunEventType is the stable SSE event name.
type RunEventType string

const (
	// EventRunStarted is emitted after the initial transaction commits.
	EventRunStarted RunEventType = "run.started"
	// EventMessageDelta carries one public text fragment.
	EventMessageDelta RunEventType = "message.delta"
	// EventReasoningDelta carries one fragment of the provider's summary of its own
	// thinking. It is not part of the answer and is never persisted.
	EventReasoningDelta RunEventType = "reasoning.delta"
	// EventToolStarted precedes one tool invocation.
	EventToolStarted RunEventType = "tool.started"
	// EventToolFinished follows one tool invocation.
	EventToolFinished RunEventType = "tool.finished"
	// EventMessageCompleted carries a persisted assistant message.
	EventMessageCompleted RunEventType = "message.completed"
	// EventRunCompleted carries a successful terminal run.
	EventRunCompleted RunEventType = "run.completed"
	// EventRunFailed carries a failed or cancelled terminal run.
	EventRunFailed RunEventType = "run.failed"
)

// RunEvent carries only the fields used by its Type; private provider metadata is absent.
type RunEvent struct {
	Type             RunEventType
	RunID, SessionID uuid.UUID
	Text             string
	ReasoningKind    ReasoningKind
	CallID, ToolName string
	DisplayName      string
	// Details carries the show_in_progress parameter values for an EventToolStarted
	// call, already resolved and rendered so a caller relays them as-is. Empty, not
	// nil, when the tool has no visible parameter.
	Details    []ToolStartedDetail
	OK         bool
	DurationMS int64
	Message    *Message
	Run        *Run
	Failure    *RunFailure
}
