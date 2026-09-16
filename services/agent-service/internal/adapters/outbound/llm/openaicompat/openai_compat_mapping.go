package openaicompat

import (
	"encoding/json"
	"math"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func mapRequest(req domain.LLMRequest) (openai.ChatCompletionNewParams, error) {
	p := openai.ChatCompletionNewParams{Model: req.Model, StreamOptions: openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)}}
	if strings.TrimSpace(req.Model) == "" {
		return p, domain.ErrProviderBadRequest
	}
	if req.SystemPrompt != "" {
		p.Messages = append(p.Messages, openai.SystemMessage(req.SystemPrompt))
	}
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			p.Messages = append(p.Messages, openai.SystemMessage(msg.Text))
		case "user":
			p.Messages = append(p.Messages, openai.UserMessage(msg.Text))
		case "assistant":
			assistant := openai.AssistantMessage(msg.Text)
			for _, call := range msg.ToolCalls {
				if call.ID == "" || call.Name == "" || !json.Valid(call.Arguments) {
					return p, domain.ErrProviderBadRequest
				}
				assistant.OfAssistant.ToolCalls = append(assistant.OfAssistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{ID: call.ID, Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{Name: call.Name, Arguments: string(call.Arguments)}}})
			}
			p.Messages = append(p.Messages, assistant)
		case "tool":
			if len(msg.ToolResults) == 0 {
				return p, domain.ErrProviderBadRequest
			}
			for _, result := range msg.ToolResults {
				if result.CallID == "" {
					return p, domain.ErrProviderBadRequest
				}
				p.Messages = append(p.Messages, openai.ToolMessage(result.Content, result.CallID))
			}
		default:
			return p, domain.ErrProviderBadRequest
		}
	}
	for _, tool := range req.Tools {
		var schema map[string]any
		if tool.Name == "" || json.Unmarshal(tool.JSONSchema, &schema) != nil || schema == nil {
			return p, domain.ErrProviderBadRequest
		}
		p.Tools = append(p.Tools, openai.ChatCompletionToolUnionParam{OfFunction: &openai.ChatCompletionFunctionToolParam{Function: shared.FunctionDefinitionParam{Name: tool.Name, Description: openai.String(tool.Description), Parameters: schema}}})
	}
	if req.Temperature != nil {
		if math.IsNaN(*req.Temperature) || math.IsInf(*req.Temperature, 0) {
			return p, domain.ErrProviderBadRequest
		}
		p.Temperature = openai.Float(*req.Temperature)
	}
	if req.MaxOutputTokens != nil {
		if *req.MaxOutputTokens < 1 {
			return p, domain.ErrProviderBadRequest
		}
		p.MaxCompletionTokens = openai.Int(int64(*req.MaxOutputTokens))
	}
	return p, nil
}
