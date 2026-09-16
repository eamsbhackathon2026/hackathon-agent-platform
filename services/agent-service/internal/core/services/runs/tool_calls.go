package runs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

const maxReplayToolCallIDLength = 40

// normalizeToolCallIDs validates provider output and repairs replay-unsafe IDs
// from OpenAI-compatible providers. Opaque provider metadata is never rewritten.
func normalizeToolCallIDs(calls []domain.ToolCall, responseMeta []byte, history []domain.ChatMessage, runID uuid.UUID, iteration int) ([]domain.ToolCall, bool) {
	seen := make(map[string]bool)
	for _, message := range history {
		for _, call := range message.ToolCalls {
			seen[call.ID] = true
		}
	}

	canRewrite := len(responseMeta) == 0
	for _, call := range calls {
		if len(call.ProviderMeta) > 0 {
			canRewrite = false
			break
		}
	}

	result := append([]domain.ToolCall(nil), calls...)
	for index := range result {
		call := &result[index]
		if !validToolCall(*call) {
			return nil, false
		}
		if seen[call.ID] {
			if !canRewrite {
				return nil, false
			}
			call.ID = uniqueToolCallID(call.ID, runID, iteration, index, seen)
		} else if canRewrite && len(call.ID) > maxReplayToolCallIDLength {
			call.ID = uniqueToolCallID(call.ID, runID, iteration, index, seen)
		}
		seen[call.ID] = true
	}
	return result, true
}

func validToolCall(call domain.ToolCall) bool {
	_, validArguments := decodeToolArguments(call.Arguments)
	return call.ID != "" && call.Name != "" && validArguments
}

func decodeToolArguments(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var arguments map[string]json.RawMessage
	err := json.Unmarshal(raw, &arguments)
	return arguments, err == nil && arguments != nil
}

func uniqueToolCallID(original string, runID uuid.UUID, iteration, index int, seen map[string]bool) string {
	for salt := 0; ; salt++ {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%d:%d", original, runID, iteration, index, salt)))
		candidate := "call_" + hex.EncodeToString(sum[:])[:maxReplayToolCallIDLength-len("call_")]
		if !seen[candidate] {
			return candidate
		}
	}
}
