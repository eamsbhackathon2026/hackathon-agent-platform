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
	// IncludeThoughts asks for thought summaries where the provider offers them.
	IncludeThoughts     bool
	Model, SystemPrompt string
	Messages            []ChatMessage
	Tools               []ToolSpec
	Temperature         *float64
	MaxOutputTokens     *int
}

// ReasoningKind says what a provider hands over when it reports its own thinking,
// because the two kinds are not interchangeable. A summary is written to be shown.
// Raw reasoning is the model talking to itself: measured on GLM over GreenNode it
// came back in English for a Vietnamese conversation, five times longer than the
// answer, and carried the draft reply the model then graded. Anything facing an
// end user wants the first and not the second.
type ReasoningKind string

// Kinds of reasoning a provider can report.
const (
	ReasoningSummary ReasoningKind = "summary"
	ReasoningRaw     ReasoningKind = "raw"
)

// LLMDelta contains public text only; tool arguments are emitted only when complete.
// Reasoning carries a provider's account of its own thinking, which is a different
// kind of text from the answer: it is never part of the reply and never persisted
// into the transcript, so a delta sets one field or the other, never both.
type LLMDelta struct {
	Text, Reasoning string
	ReasoningKind   ReasoningKind
}

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
