package runs

import (
	"math"

	"agent-platform/services/agent-service/internal/core/domain"
)

const (
	softInputPercent        = 75
	maxCalibrationFactor    = 3.0
	recentToolMessagesKept  = 3
	prunedToolResultContent = "[Earlier tool result omitted from this request; the durable transcript is unchanged.]"
)

type contextBudget struct {
	rawEstimate      int
	adjustedEstimate int
	softInputLimit   int
	hardInputLimit   int
}

func (b contextBudget) overSoft() bool { return b.adjustedEstimate > b.softInputLimit }
func (b contextBudget) overHard() bool { return b.adjustedEstimate > b.hardInputLimit }

type contextBudgeter struct{ calibrationFactor float64 }

func newContextBudgeter(session domain.Session) contextBudgeter {
	factor := 1.0
	if session.LastPromptEstimatedTokens != nil && session.LastPromptTokens != nil && *session.LastPromptEstimatedTokens > 0 {
		observed := float64(*session.LastPromptTokens) / float64(*session.LastPromptEstimatedTokens)
		if observed > factor {
			factor = math.Min(observed, maxCalibrationFactor)
		}
	}
	return contextBudgeter{calibrationFactor: factor}
}

func (b *contextBudgeter) observe(rawEstimate, actual int) {
	if rawEstimate < 1 || actual <= rawEstimate {
		return
	}
	observed := math.Min(float64(actual)/float64(rawEstimate), maxCalibrationFactor)
	if observed > b.calibrationFactor {
		b.calibrationFactor = observed
	}
}

func (b contextBudgeter) assess(agent domain.Agent, messages []domain.ChatMessage, tools []domain.ToolSpec) contextBudget {
	window := agent.ContextWindowTokens
	if window == 0 {
		window = domain.DefaultContextWindowTokens
	}
	reserve := domain.EffectiveMaxOutputTokens(agent)
	safety := domain.ContextSafetyTokens(window)
	hard := window - reserve - safety
	if hard < 0 {
		hard = 0
	}
	raw := estimateLLMInput(domain.LLMRequest{Model: agent.Model, SystemPrompt: agent.SystemPrompt, Messages: messages, Tools: tools})
	return contextBudget{rawEstimate: raw, adjustedEstimate: int(math.Ceil(float64(raw) * b.calibrationFactor)), softInputLimit: hard * softInputPercent / 100, hardInputLimit: hard}
}

func estimateLLMInput(request domain.LLMRequest) int {
	tokens := 12 + estimateBytes(len(request.SystemPrompt))
	for _, tool := range request.Tools {
		tokens += 20 + estimateBytes(len(tool.Name)+len(tool.Description)+len(tool.JSONSchema))
	}
	for _, message := range request.Messages {
		tokens += 8 + estimateBytes(len(message.Role)+len(message.Text)+len(message.ProviderMeta))
		for _, call := range message.ToolCalls {
			tokens += 12 + estimateBytes(len(call.ID)+len(call.Name)+len(call.Arguments)+len(call.ProviderMeta))
		}
		for _, result := range message.ToolResults {
			tokens += 10 + estimateBytes(len(result.CallID)+len(result.Name)+len(result.Content))
		}
	}
	if tokens < 1 {
		return 1
	}
	return tokens
}

func estimateBytes(size int) int {
	return (size + 2) / 3
}

func pruneOldToolResults(messages []domain.ChatMessage) []domain.ChatMessage {
	result := cloneChatMessages(messages)
	remaining := recentToolMessagesKept
	for index := len(result) - 1; index >= 0; index-- {
		if result[index].Role != "tool" {
			continue
		}
		if remaining > 0 {
			remaining--
			continue
		}
		for resultIndex := range result[index].ToolResults {
			result[index].ToolResults[resultIndex].Content = prunedToolResultContent
			result[index].ToolResults[resultIndex].Truncated = true
		}
	}
	return result
}

func cloneChatMessages(messages []domain.ChatMessage) []domain.ChatMessage {
	result := make([]domain.ChatMessage, len(messages))
	for index, message := range messages {
		result[index] = message
		result[index].ToolCalls = append([]domain.ToolCall(nil), message.ToolCalls...)
		for callIndex := range result[index].ToolCalls {
			result[index].ToolCalls[callIndex].Arguments = append([]byte(nil), message.ToolCalls[callIndex].Arguments...)
			result[index].ToolCalls[callIndex].ProviderMeta = append([]byte(nil), message.ToolCalls[callIndex].ProviderMeta...)
		}
		result[index].ToolResults = append([]domain.ToolResult(nil), message.ToolResults...)
		result[index].ProviderMeta = append([]byte(nil), message.ProviderMeta...)
	}
	return result
}
