package gemini

import (
	"context"
	"encoding/json"
	"iter"

	"agent-platform/services/agent-service/internal/core/domain"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

// Stream emits public text and retains complete ordered parts for subsequent turns.
func (c *Client) Stream(ctx context.Context, req domain.LLMRequest, onDelta func(domain.LLMDelta)) (domain.LLMResult, error) {
	result := domain.LLMResult{}
	contents, config, err := mapRequest(req)
	if err != nil {
		return result, err
	}
	meta := replayMetadata{Version: 1}
	seen := map[string]bool{}
	next, stop := iter.Pull2(c.sdk.Models.GenerateContentStream(ctx, req.Model, contents, config))
	defer stop()
	for {
		chunk, ok, err := nextResponse(next)
		if !ok {
			break
		}
		if err != nil {
			return domain.LLMResult{}, safeError(ctx, err, false)
		}
		if chunk == nil || (len(chunk.Candidates) == 0 && chunk.UsageMetadata == nil && chunk.PromptFeedback == nil) {
			return domain.LLMResult{}, domain.ErrProviderBadRequest
		}
		if usage := chunk.UsageMetadata; usage != nil {
			input, output := int(usage.PromptTokenCount), int(usage.CandidatesTokenCount)
			result.Usage = domain.TokenUsage{InputTokens: &input, OutputTokens: &output}
		}
		if feedback := chunk.PromptFeedback; feedback != nil && feedback.BlockReason != "" && feedback.BlockReason != genai.BlockedReasonUnspecified {
			result.FinishReason = string(feedback.BlockReason)
		}
		for _, candidate := range chunk.Candidates {
			if candidate == nil || candidate.Index != 0 {
				continue
			}
			if candidate.FinishReason != "" && candidate.FinishReason != genai.FinishReasonUnspecified {
				result.FinishReason = string(candidate.FinishReason)
			}
			if candidate.Content == nil {
				continue
			}
			for _, part := range candidate.Content.Parts {
				if part == nil {
					return domain.LLMResult{}, domain.ErrProviderBadRequest
				}
				index := len(meta.Parts)
				meta.Parts = append(meta.Parts, part)
				if !part.Thought && part.Text != "" {
					result.Text += part.Text
					if onDelta != nil {
						onDelta(domain.LLMDelta{Text: part.Text})
					}
				}
				if call := part.FunctionCall; call != nil {
					if call.Name == "" || len(call.PartialArgs) > 0 || (call.WillContinue != nil && *call.WillContinue) {
						return domain.LLMResult{}, domain.ErrProviderBadRequest
					}
					id := call.ID
					if id == "" {
						id = "call_" + uuid.NewString()
					}
					if seen[id] {
						return domain.LLMResult{}, domain.ErrProviderBadRequest
					}
					seen[id] = true
					args := call.Args
					if args == nil {
						args = map[string]any{}
					}
					arguments, err := json.Marshal(args)
					if err != nil {
						return domain.LLMResult{}, domain.ErrProviderBadRequest
					}
					binding := callBinding{CallID: id, OriginalID: call.ID, PartIndex: index}
					meta.Calls = append(meta.Calls, binding)
					binding.PartIndex = 0
					private, err := json.Marshal(replayMetadata{Version: 1, Parts: []*genai.Part{part}, Calls: []callBinding{binding}})
					if err != nil {
						return domain.LLMResult{}, domain.ErrProviderBadRequest
					}
					result.ToolCalls = append(result.ToolCalls, domain.ToolCall{ID: id, Name: call.Name, Arguments: arguments, ProviderMeta: private})
				}
			}
		}
	}
	if ctx.Err() != nil {
		return domain.LLMResult{}, ctx.Err()
	}
	if result.FinishReason == "" {
		return domain.LLMResult{}, domain.ErrProviderUnreachable
	}
	if len(meta.Parts) > 0 {
		result.ProviderMeta, err = json.Marshal(meta)
		if err != nil {
			return domain.LLMResult{}, domain.ErrProviderBadRequest
		}
	}
	return result, nil
}

// The pinned SDK converter uses type assertions on untrusted JSON. Pulling one
// response here isolates its panic recovery from application onDelta callbacks.
func nextResponse(next func() (*genai.GenerateContentResponse, error, bool)) (chunk *genai.GenerateContentResponse, ok bool, err error) {
	defer func() {
		if recover() != nil {
			chunk, err, ok = nil, domain.ErrProviderBadRequest, true
		}
	}()
	chunk, err, ok = next()
	return chunk, ok, err
}
