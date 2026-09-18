package openaicompat

import (
	"context"
	"encoding/json"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/respjson"
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
			if choice.Index != 0 || onDelta == nil {
				continue
			}
			if thought := reasoningDelta(choice.Delta.JSON.ExtraFields); thought != "" {
				onDelta(domain.LLMDelta{Reasoning: thought, ReasoningKind: domain.ReasoningRaw})
			}
			if choice.Delta.Content != "" {
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

// reasoningDelta reads a thinking model's own account of its reasoning.
//
// The field sits outside the OpenAI wire format, so the SDK keeps it among the
// unknown fields rather than on the struct. Deployments disagree on the name:
// DeepSeek, GLM and Qwen send reasoning_content, while several vLLM builds send
// reasoning, so both are read. The run engine decides whether any of it reaches
// a caller — unlike Gemini there is no request flag to turn it off, so a model
// that always narrates its thinking would otherwise narrate it to everyone.
// debugExtras is a test seam, nil in production. Which unknown fields a
// deployment actually sends can only be learned by asking it, so the live probe
// in this package reports them instead of leaving a third spelling invisible.
var debugExtras func(map[string]string)

func reasoningDelta(extra map[string]respjson.Field) string {
	if debugExtras != nil && len(extra) > 0 {
		fields := make(map[string]string, len(extra))
		for name, value := range extra {
			fields[name] = value.Raw()
		}
		debugExtras(fields)
	}
	for _, name := range []string{"reasoning_content", "reasoning"} {
		field, ok := extra[name]
		if !ok {
			continue
		}
		// Unmarshal is the only check that matters here: an extra field carries no
		// status the SDK tracks, so Valid reports false even for a good value.
		var text string
		if err := json.Unmarshal([]byte(field.Raw()), &text); err == nil && text != "" {
			return text
		}
	}
	return ""
}
