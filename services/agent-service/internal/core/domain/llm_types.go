package domain

import "encoding/json"

// ToolSpec is the provider-neutral JSON Schema declaration of one tool. Name is
// generated for the provider and may carry a suffix that disambiguates a collision; Ref
// is the identity an operator sees on the Tools screen and the identity a skill writes
// after its $ sigil, so the two stay separable when a skill's instructions are rewritten
// for the run.
type ToolSpec struct {
	Name, Ref, Description string
	// DisplayName is the end-user label for this tool while it runs, already
	// resolved from the saved labels so the run engine does not repeat that choice.
	DisplayName string
	JSONSchema  json.RawMessage
}

// ToolCall preserves provider metadata privately for subsequent turns.
type ToolCall struct {
	ID, Name     string
	Arguments    json.RawMessage
	ProviderMeta []byte
}

// ToolResult supplies the response to an earlier tool call.
type ToolResult struct {
	CallID, Name, Content string
	IsError               bool
	Truncated             bool
	// StatusCode is the HTTP status for tools backed by an HTTP call, nil for MCP
	// tools and for failures that never reached the target. It travels with the
	// result so the model and the activity timeline can tell a rejected request
	// apart from a broken target system.
	StatusCode *int
}

// ChatMessage carries conversation text and tool exchanges. ProviderMeta preserves
// the complete ordered assistant response, including signatures on non-tool parts.
type ChatMessage struct {
	Role, Text   string
	ToolCalls    []ToolCall
	ToolResults  []ToolResult
	ProviderMeta []byte
}

// LLMRequest is shared by synchronous collection and streaming generation.
type LLMRequest struct {
	Model, SystemPrompt string
	Messages            []ChatMessage
	Tools               []ToolSpec
	Temperature         *float64
	MaxOutputTokens     *int
}

// LLMDelta contains public text only; tool arguments are emitted only when complete.
type LLMDelta struct{ Text string }

// TokenUsage preserves missing provider usage as nil rather than inventing zeros.
type TokenUsage struct{ InputTokens, OutputTokens *int }

// LLMResult contains a completed response and private replay metadata.
type LLMResult struct {
	Text         string
	ToolCalls    []ToolCall
	Usage        TokenUsage
	FinishReason string
	ProviderMeta []byte
}
