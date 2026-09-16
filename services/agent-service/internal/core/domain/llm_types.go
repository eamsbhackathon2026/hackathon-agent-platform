package domain

import "encoding/json"

// ToolSpec is the provider-neutral JSON Schema declaration of one tool.
type ToolSpec struct {
	Name, Description string
	JSONSchema        json.RawMessage
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
