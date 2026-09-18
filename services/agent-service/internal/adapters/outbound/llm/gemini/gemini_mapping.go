package gemini

import (
	"encoding/json"
	"math"

	"agent-platform/services/agent-service/internal/core/domain"
	"google.golang.org/genai"
)

func mapRequest(req domain.LLMRequest) ([]*genai.Content, *genai.GenerateContentConfig, error) {
	config := &genai.GenerateContentConfig{HTTPOptions: &genai.HTTPOptions{ExtrasRequestProvider: restoreEmptyTextParts}}
	// Thought summaries cost output tokens and not every product wants to narrate
	// what the assistant is thinking, so the agent decides. Without this the model
	// still thinks but says nothing about it, and a caller that wants to show
	// progress has only tool calls to go by. What comes back is Google's summary of
	// the thinking, not the raw chain of thought — the part fit to show a customer.
	if req.IncludeThoughts {
		config.ThinkingConfig = &genai.ThinkingConfig{IncludeThoughts: true}
	}
	if req.Model == "" {
		return nil, nil, domain.ErrProviderBadRequest
	}
	if req.SystemPrompt != "" {
		config.SystemInstruction = genai.NewContentFromText(req.SystemPrompt, genai.RoleUser)
	}
	if req.Temperature != nil {
		if math.IsNaN(*req.Temperature) || math.IsInf(*req.Temperature, 0) || *req.Temperature < 0 || *req.Temperature > math.MaxFloat32 {
			return nil, nil, domain.ErrProviderBadRequest
		}
		temperature := float32(*req.Temperature)
		config.Temperature = &temperature
	}
	if req.MaxOutputTokens != nil {
		if *req.MaxOutputTokens < 1 || *req.MaxOutputTokens > math.MaxInt32 {
			return nil, nil, domain.ErrProviderBadRequest
		}
		config.MaxOutputTokens = int32(*req.MaxOutputTokens)
	}
	for _, tool := range req.Tools {
		var schema map[string]any
		if tool.Name == "" || json.Unmarshal(tool.JSONSchema, &schema) != nil || schema == nil {
			return nil, nil, domain.ErrProviderBadRequest
		}
		config.Tools = append(config.Tools, &genai.Tool{FunctionDeclarations: []*genai.FunctionDeclaration{{Name: tool.Name, Description: tool.Description, ParametersJsonSchema: schema}}})
	}
	contents := make([]*genai.Content, 0, len(req.Messages))
	bindings := map[string]responseBinding{}
	for _, message := range req.Messages {
		content, err := mapMessage(message, bindings)
		if err != nil {
			return nil, nil, err
		}
		contents = append(contents, content)
	}
	return contents, config, nil
}

func mapMessage(message domain.ChatMessage, bindings map[string]responseBinding) (*genai.Content, error) {
	content := &genai.Content{}
	switch message.Role {
	case "assistant":
		content.Role = "model"
	case "user", "tool":
		content.Role = "user"
	default:
		return nil, domain.ErrProviderBadRequest
	}
	if len(message.ProviderMeta) > 0 {
		if message.Role != "assistant" {
			return nil, domain.ErrProviderBadRequest
		}
		meta, err := readMetadata(message.ProviderMeta)
		if err != nil {
			return nil, err
		}
		if err := registerMetadata(meta, bindings); err != nil {
			return nil, err
		}
		content.Parts = meta.Parts
		return content, nil
	}
	if message.Text != "" {
		content.Parts = append(content.Parts, genai.NewPartFromText(message.Text))
	}
	for _, call := range message.ToolCalls {
		if message.Role != "assistant" || call.ID == "" || call.Name == "" {
			return nil, domain.ErrProviderBadRequest
		}
		if len(call.ProviderMeta) > 0 {
			meta, err := readMetadata(call.ProviderMeta)
			if err != nil {
				return nil, err
			}
			if len(meta.Calls) != 1 || len(meta.Parts) != 1 || meta.Calls[0].CallID != call.ID || meta.Parts[0].FunctionCall.Name != call.Name {
				return nil, domain.ErrProviderBadRequest
			}
			if err := registerMetadata(meta, bindings); err != nil {
				return nil, err
			}
			content.Parts = append(content.Parts, meta.Parts...)
			continue
		}
		var args map[string]any
		if json.Unmarshal(call.Arguments, &args) != nil || args == nil {
			return nil, domain.ErrProviderBadRequest
		}
		if _, exists := bindings[call.ID]; exists {
			return nil, domain.ErrProviderBadRequest
		}
		bindings[call.ID] = responseBinding{id: call.ID, name: call.Name}
		content.Parts = append(content.Parts, &genai.Part{FunctionCall: &genai.FunctionCall{ID: call.ID, Name: call.Name, Args: args}})
	}
	for _, result := range message.ToolResults {
		binding, found := bindings[result.CallID]
		if message.Role != "tool" || !found || result.Name != binding.name {
			return nil, domain.ErrProviderBadRequest
		}
		key := "output"
		if result.IsError {
			key = "error"
		}
		content.Parts = append(content.Parts, &genai.Part{FunctionResponse: &genai.FunctionResponse{ID: binding.id, Name: binding.name, Response: map[string]any{key: result.Content}}})
	}
	return content, nil
}
