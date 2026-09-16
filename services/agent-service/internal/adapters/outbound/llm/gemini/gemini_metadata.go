package gemini

import (
	"encoding/json"

	"agent-platform/services/agent-service/internal/core/domain"
	"google.golang.org/genai"
)

// replayMetadata retains ordered SDK parts, including signatures on text and empty
// parts. Unknown wire fields not represented by the pinned SDK cannot be preserved.
type replayMetadata struct {
	Version int           `json:"version"`
	Parts   []*genai.Part `json:"parts"`
	Calls   []callBinding `json:"calls"`
}

type callBinding struct {
	CallID     string `json:"call_id"`
	OriginalID string `json:"original_id"`
	PartIndex  int    `json:"part_index"`
}

type responseBinding struct{ id, name string }

func readMetadata(raw []byte) (replayMetadata, error) {
	var meta replayMetadata
	if json.Unmarshal(raw, &meta) != nil || meta.Version != 1 || len(meta.Parts) == 0 {
		return meta, domain.ErrProviderBadRequest
	}
	seen := map[string]bool{}
	indices := map[int]bool{}
	for _, binding := range meta.Calls {
		if binding.CallID == "" || seen[binding.CallID] || binding.PartIndex < 0 || binding.PartIndex >= len(meta.Parts) || indices[binding.PartIndex] {
			return meta, domain.ErrProviderBadRequest
		}
		part := meta.Parts[binding.PartIndex]
		if part == nil || part.FunctionCall == nil || part.FunctionCall.ID != binding.OriginalID || part.FunctionCall.Name == "" {
			return meta, domain.ErrProviderBadRequest
		}
		seen[binding.CallID], indices[binding.PartIndex] = true, true
	}
	for index, part := range meta.Parts {
		if part == nil || (part.FunctionCall != nil && !indices[index]) {
			return meta, domain.ErrProviderBadRequest
		}
	}
	return meta, nil
}

func registerMetadata(meta replayMetadata, bindings map[string]responseBinding) error {
	for _, call := range meta.Calls {
		if _, exists := bindings[call.CallID]; exists {
			return domain.ErrProviderBadRequest
		}
		bindings[call.CallID] = responseBinding{id: call.OriginalID, name: meta.Parts[call.PartIndex].FunctionCall.Name}
	}
	return nil
}
