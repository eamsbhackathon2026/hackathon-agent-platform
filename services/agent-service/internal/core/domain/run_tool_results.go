package domain

import "encoding/json"

// RunToolResult is one tool invocation as seen by an integrating system: what the
// model asked for and what the tool answered, in execution order. It is derived
// from the transcript rather than stored, so it never drifts from the messages.
type RunToolResult struct {
	CallID, ToolName string
	Arguments        json.RawMessage
	// Result holds the tool body as JSON when it parsed, otherwise the raw text
	// encoded as a JSON string, so consumers always receive valid JSON.
	Result  json.RawMessage
	IsError bool
}

// ToolResultsFromMessages pairs each tool message with the assistant call that
// requested it. Messages must be in transcript order. A tool message whose call
// cannot be found still appears, named by the message, with empty arguments.
func ToolResultsFromMessages(messages []Message) []RunToolResult {
	calls := make(map[string]ToolCall)
	results := []RunToolResult{}
	for _, message := range messages {
		for _, call := range message.ToolCalls {
			calls[call.ID] = call
		}
		if message.Role != "tool" || message.ToolCallID == nil {
			continue
		}
		result := RunToolResult{CallID: *message.ToolCallID, Arguments: json.RawMessage(`{}`), Result: toolResultJSON(message.Content), IsError: message.IsError}
		if call, ok := calls[*message.ToolCallID]; ok {
			result.ToolName = call.Name
			if len(call.Arguments) > 0 {
				result.Arguments = call.Arguments
			}
		} else if message.ToolName != nil {
			result.ToolName = *message.ToolName
		}
		results = append(results, result)
	}
	return results
}

func toolResultJSON(content string) json.RawMessage {
	if json.Valid([]byte(content)) {
		return json.RawMessage(content)
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return encoded
}
