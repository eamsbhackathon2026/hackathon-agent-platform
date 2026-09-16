// Package delivery dispatches terminal run callbacks.
package delivery

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

type webhookRun struct {
	ID                  uuid.UUID           `json:"id"`
	AgentID             uuid.UUID           `json:"agent_id"`
	SessionID           uuid.UUID           `json:"session_id"`
	Mode                domain.RunMode      `json:"mode"`
	Status              domain.RunStatus    `json:"status"`
	Source              domain.RunSource    `json:"source"`
	TriggeredByUserID   *uuid.UUID          `json:"triggered_by_user_id"`
	TriggeredByAPIKeyID *uuid.UUID          `json:"triggered_by_api_key_id"`
	Input               domain.RunInput     `json:"input"`
	Output              *string             `json:"output"`
	Error               *webhookError       `json:"error"`
	Iterations          int                 `json:"iterations"`
	Usage               webhookUsage        `json:"usage"`
	Metadata            json.RawMessage     `json:"metadata"`
	CancelRequestedAt   *time.Time          `json:"cancel_requested_at"`
	QueuedAt            *time.Time          `json:"queued_at"`
	StartedAt           *time.Time          `json:"started_at"`
	FinishedAt          *time.Time          `json:"finished_at"`
	CreatedAt           time.Time           `json:"created_at"`
	ToolResults         []webhookToolResult `json:"tool_results"`
}

type webhookToolResult struct {
	CallID    string          `json:"call_id"`
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
	Result    json.RawMessage `json:"result"`
	IsError   bool            `json:"is_error"`
}

type webhookError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type webhookUsage struct {
	InputTokens  *int `json:"input_tokens"`
	OutputTokens *int `json:"output_tokens"`
}

type webhookPayload struct {
	Type      domain.WebhookEvent `json:"type"`
	Timestamp time.Time           `json:"timestamp"`
	Data      webhookRun          `json:"data"`
}

// BuildPayload serializes the public API shape without adapter dependencies.
func BuildPayload(run domain.Run, at time.Time) ([]byte, error) {
	var failure *webhookError
	if run.Failure != nil {
		failure = &webhookError{Code: run.Failure.Code, Message: run.Failure.Message}
	}
	toolResults := make([]webhookToolResult, 0, len(run.ToolResults))
	for _, item := range run.ToolResults {
		toolResults = append(toolResults, webhookToolResult{CallID: item.CallID, ToolName: item.ToolName, Arguments: item.Arguments, Result: item.Result, IsError: item.IsError})
	}
	data := webhookRun{ID: run.ID, AgentID: run.AgentID, SessionID: run.SessionID, Mode: run.Mode, Status: run.Status, Source: run.Source, TriggeredByUserID: run.TriggeredByUserID, TriggeredByAPIKeyID: run.TriggeredByAPIKeyID, Input: run.Input, Output: run.Output, Error: failure, Iterations: run.Iterations, Usage: webhookUsage{InputTokens: run.Usage.InputTokens, OutputTokens: run.Usage.OutputTokens}, Metadata: run.Metadata, CancelRequestedAt: run.CancelRequestedAt, QueuedAt: run.QueuedAt, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, CreatedAt: run.CreatedAt, ToolResults: toolResults}
	return json.Marshal(webhookPayload{Type: domain.TerminalWebhookEvent(run.Status), Timestamp: at, Data: data})
}
