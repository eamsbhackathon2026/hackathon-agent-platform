package gemini

// restoreEmptyTextParts restores the data oneof after the SDK omits Text == "".
// Streaming can end with empty text, with or without a thought signature. Keep
// these parts in place: moving a signature to another part invalidates replay.
func restoreEmptyTextParts(body map[string]any) map[string]any {
	contents, _ := body["contents"].([]map[string]any)
	for _, content := range contents {
		if content["role"] != "model" {
			continue
		}
		parts, _ := content["parts"].([]map[string]any)
		for _, part := range parts {
			if part != nil && !hasPartData(part) {
				part["text"] = ""
			}
		}
	}
	return body
}

func hasPartData(part map[string]any) bool {
	for _, field := range []string{"text", "functionCall", "functionResponse", "inlineData", "fileData", "executableCode", "codeExecutionResult", "toolCall", "toolResponse"} {
		if _, present := part[field]; present {
			return true
		}
	}
	return false
}
