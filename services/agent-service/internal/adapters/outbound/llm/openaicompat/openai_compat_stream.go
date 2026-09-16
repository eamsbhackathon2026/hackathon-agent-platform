package openaicompat

import (
	"context"
	"encoding/json"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
	"github.com/openai/openai-go/v3"
)

// Stream emits text deltas and returns tool calls only after successful completion.
func (c *Client) Stream(ctx context.Context, req domain.LLMRequest, onDelta func(domain.LLMDelta)) (domain.LLMResult, error) {
	var result domain.LLMResult
	params, err := mapRequest(req)
	if err != nil {
		return result, err
	}
	stream := c.sdk.Chat.Completions.NewStreaming(ctx, params)
	defer func() { _ = stream.Close() }()
	var acc openai.ChatCompletionAccumulator
	for stream.Next() {
		chunk := stream.Current()
		if !acc.AddChunk(chunk) {
			return domain.LLMResult{}, domain.ErrProviderUnreachable
		}
		// SDK field metadata distinguishes a real zero from absent or null usage.
		if chunk.JSON.Usage.Valid() {
			if chunk.Usage.JSON.PromptTokens.Valid() {
				v := int(chunk.Usage.PromptTokens)
				result.Usage.InputTokens = &v
			}
			if chunk.Usage.JSON.CompletionTokens.Valid() {
				v := int(chunk.Usage.CompletionTokens)
				result.Usage.OutputTokens = &v
			}
		}
		for _, choice := range chunk.Choices {
			if choice.Index == 0 && choice.Delta.Content != "" && onDelta != nil {
				onDelta(domain.LLMDelta{Text: choice.Delta.Content})
			}
		}
	}
	if err := stream.Err(); err != nil {
		return domain.LLMResult{}, providerError(ctx, err, false)
	}
	if err := ctx.Err(); err != nil {
		return domain.LLMResult{}, err
	}
	if len(acc.Choices) == 0 || acc.Choices[0].FinishReason == "" {
		return domain.LLMResult{}, domain.ErrProviderUnreachable
	}
	choice := acc.Choices[0]
	result.Text = choice.Message.Content
	result.FinishReason = strings.ToLower(strings.TrimSpace(choice.FinishReason))
	for _, call := range choice.Message.ToolCalls {
		if call.ID == "" || call.Function.Name == "" || !json.Valid([]byte(call.Function.Arguments)) {
			return domain.LLMResult{}, domain.ErrProviderUnreachable
		}
		result.ToolCalls = append(result.ToolCalls, domain.ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: json.RawMessage(call.Function.Arguments)})
	}
	return result, nil
}
