package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// RunMode selects how a caller receives one assistant execution.
type RunMode string

const (
	// RunModeSync waits for a JSON result in the request.
	RunModeSync RunMode = "sync"
	// RunModeStream emits incremental SSE events.
	RunModeStream RunMode = "stream"
	// RunModeAsync is queued for background execution.
	RunModeAsync RunMode = "async"
)

// RunStatus is the durable lifecycle state of an execution.
type RunStatus string

const (
	// RunQueued is waiting for an asynchronous worker.
	RunQueued RunStatus = "queued"
	// RunRunning is actively executing.
	RunRunning RunStatus = "running"
	// RunSucceeded completed with an assistant result.
	RunSucceeded RunStatus = "succeeded"
	// RunFailed ended with a stable failure.
	RunFailed RunStatus = "failed"
	// RunCancelled stopped at a caller or durable cancellation request.
	RunCancelled RunStatus = "cancelled"
)

// RunSource identifies the caller-facing product surface.
type RunSource string

const (
	// RunSourcePlayground identifies a signed-in user request.
	RunSourcePlayground RunSource = "playground"
	// RunSourceAPI identifies an API-key request.
	RunSourceAPI RunSource = "api"
)

// RunInput is persisted separately from the generated provider prompt.
type RunInput struct {
	Message string `json:"message"`
}

// RunFailure contains only a stable public code and a safe explanation.
type RunFailure struct{ Code, Message string }

// Run is one durable execution and its accumulated result.
type Run struct {
	ID, AgentID, SessionID                 uuid.UUID
	Mode                                   RunMode
	Status                                 RunStatus
	Source                                 RunSource
	TriggeredByUserID, TriggeredByAPIKeyID *uuid.UUID
	Input                                  RunInput
	Output                                 *string
	Failure                                *RunFailure
	Iterations                             int
	Usage                                  TokenUsage
	Metadata                               json.RawMessage
	WebhookURL                             *string
	CancelRequestedAt, QueuedAt            *time.Time
	StartedAt, FinishedAt                  *time.Time
	CreatedAt                              time.Time
	// ToolResults is filled from the transcript for run detail; nil means not loaded.
	ToolResults []RunToolResult
}

// Terminal reports whether no worker may change the run lifecycle again.
func (r Run) Terminal() bool {
	return r.Status == RunSucceeded || r.Status == RunFailed || r.Status == RunCancelled
}

// Transition rejects impossible lifecycle changes.
func (r *Run) Transition(next RunStatus) error {
	valid := (r.Status == RunQueued && (next == RunRunning || next == RunFailed || next == RunCancelled)) ||
		(r.Status == RunRunning && (next == RunSucceeded || next == RunFailed || next == RunCancelled))
	if !valid {
		return errors.New("invalid run status transition")
	}
	r.Status = next
	return nil
}

// OwnerID returns the identity that owns this run.
func (r Run) OwnerID() *uuid.UUID {
	if r.TriggeredByAPIKeyID != nil {
		return r.TriggeredByAPIKeyID
	}
	return r.TriggeredByUserID
}
