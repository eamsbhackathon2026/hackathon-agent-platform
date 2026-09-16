package runs

import (
	"encoding/json"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
)

const (
	maxToolCallsPerRun     = 25
	maxEmptyReplyRetries   = 2
	maxToolResponseRetries = 2

	wrapUpInstruction         = "[System] You have used most of the available processing steps. Start wrapping up and prepare a concise final answer."
	urgentWrapUpInstruction   = "[System] You are close to the processing limit. Finish the current work and answer the user as soon as possible."
	emptyReplyInstruction     = "[System] Your response was empty. Give the user a visible, useful answer now."
	toolRetryInstruction      = "[System] One or more tool call arguments were incomplete or malformed. Retry with valid, shorter arguments."
	toolCallBudgetInstruction = "[System] The tool-call budget is exhausted. Use the available results to answer the user now without calling more tools."
)

type iterationPolicy struct {
	emptyReplyRetries   int
	toolResponseRetries int
	wrapUpSent          bool
	urgentWrapUpSent    bool
	forceFinal          bool
	nextContext         []domain.ChatMessage
}

func (p *iterationPolicy) prepareRequest(history []domain.ChatMessage, specs []domain.ToolSpec, iteration, maxIterations, totalToolCalls int) ([]domain.ChatMessage, []domain.ToolSpec, bool) {
	tools := specs
	hasTools := len(specs) > 0
	finalRequest := p.forceFinal || totalToolCalls >= maxToolCallsPerRun || (hasTools && iteration == maxIterations)
	instruction := ""
	if finalRequest {
		tools = nil
		if p.forceFinal || totalToolCalls >= maxToolCallsPerRun {
			instruction = toolCallBudgetInstruction
		} else {
			instruction = finalIterationInstruction
		}
	} else if hasTools {
		instruction = p.iterationBudgetInstruction(iteration, maxIterations)
	}

	if len(p.nextContext) == 0 && instruction == "" {
		return history, tools, finalRequest
	}
	messages := append([]domain.ChatMessage(nil), history...)
	messages = append(messages, p.nextContext...)
	p.nextContext = nil
	if instruction != "" {
		messages = append(messages, domain.ChatMessage{Role: "user", Text: instruction})
	}
	return messages, tools, finalRequest
}

func (p *iterationPolicy) iterationBudgetInstruction(iteration, maxIterations int) string {
	if maxIterations <= 0 {
		return ""
	}
	if !p.urgentWrapUpSent && iteration*100 >= maxIterations*90 {
		p.urgentWrapUpSent = true
		return urgentWrapUpInstruction
	}
	if !p.wrapUpSent && iteration*100 >= maxIterations*70 {
		p.wrapUpSent = true
		return wrapUpInstruction
	}
	return ""
}

func (p *iterationPolicy) inspectResponse(result domain.LLMResult, specs []domain.ToolSpec, iteration, maxIterations int) (bool, *domain.RunFailure) {
	if len(result.ToolCalls) > 0 && toolCallsNeedRetry(result, specs) {
		if iteration >= maxIterations || p.toolResponseRetries >= maxToolResponseRetries {
			return false, &domain.RunFailure{Code: "validation_failed", Message: "Kết nối AI liên tục trả về tham số công cụ không đầy đủ."}
		}
		p.toolResponseRetries++
		p.scheduleRetry(result, toolRetryInstruction, false)
		return true, nil
	}
	p.toolResponseRetries = 0

	if len(result.ToolCalls) == 0 && strings.TrimSpace(result.Text) == "" {
		if iteration >= maxIterations || p.emptyReplyRetries >= maxEmptyReplyRetries {
			return false, &domain.RunFailure{Code: "validation_failed", Message: "Kết nối AI không tạo được nội dung trả lời."}
		}
		p.emptyReplyRetries++
		p.scheduleRetry(result, emptyReplyInstruction, true)
		return true, nil
	}
	p.emptyReplyRetries = 0
	return false, nil
}

func (p *iterationPolicy) scheduleRetry(result domain.LLMResult, instruction string, preserveMetadata bool) {
	if strings.TrimSpace(result.Text) != "" || (preserveMetadata && len(result.ProviderMeta) > 0) {
		message := domain.ChatMessage{Role: "assistant", Text: result.Text}
		if preserveMetadata {
			message.ProviderMeta = append([]byte(nil), result.ProviderMeta...)
		}
		p.nextContext = append(p.nextContext, message)
	}
	p.nextContext = append(p.nextContext, domain.ChatMessage{Role: "user", Text: instruction})
}

func (p *iterationPolicy) scheduleInstruction(instruction string) {
	if instruction != "" {
		p.nextContext = append(p.nextContext, domain.ChatMessage{Role: "user", Text: instruction})
	}
}

func (p *iterationPolicy) preservePartialText(result domain.LLMResult) {
	if strings.TrimSpace(result.Text) != "" {
		p.nextContext = append(p.nextContext, domain.ChatMessage{Role: "assistant", Text: result.Text})
	}
}

func toolCallsNeedRetry(result domain.LLMResult, specs []domain.ToolSpec) bool {
	if finishReasonTruncated(result.FinishReason) {
		return true
	}
	schemas := make(map[string]json.RawMessage, len(specs))
	for _, spec := range specs {
		schemas[spec.Name] = spec.JSONSchema
	}
	for _, call := range result.ToolCalls {
		arguments, valid := decodeToolArguments(call.Arguments)
		if !valid {
			return true
		}
		var schema struct {
			Required []string `json:"required"`
		}
		if raw, found := schemas[call.Name]; !found || json.Unmarshal(raw, &schema) != nil {
			continue
		}
		for _, required := range schema.Required {
			if _, found := arguments[required]; !found {
				return true
			}
		}
	}
	return false
}

func finishReasonTruncated(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "length", "max_tokens", "max_output_tokens":
		return true
	default:
		return false
	}
}

func loopWarningInstruction(call domain.ToolCall, observation domain.LoopObservation) string {
	if !observation.Warning {
		return ""
	}
	if observation.RepetitionCount > 1 {
		return "[System] Tool " + call.Name + " is returning the same result for the same arguments. Change approach or answer with the information already available."
	}
	return "[System] Tool " + call.Name + " keeps returning the same result for different arguments. Stop varying inputs without progress; change approach or answer the user."
}
