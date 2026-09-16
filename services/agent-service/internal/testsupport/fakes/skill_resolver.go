package fakes

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// SkillResolver returns a scripted prompt and records run-start resolutions.
type SkillResolver struct {
	mu      sync.Mutex
	Prompt  string
	Err     error
	Resolve func(context.Context, uuid.UUID, string) (string, error)
	Calls   int
}

// ResolveSystemPrompt returns Prompt when configured, otherwise the base prompt.
func (r *SkillResolver) ResolveSystemPrompt(ctx context.Context, agentID uuid.UUID, base string) (string, error) {
	r.mu.Lock()
	r.Calls++
	resolve, prompt, err := r.Resolve, r.Prompt, r.Err
	r.mu.Unlock()
	if resolve != nil {
		return resolve(ctx, agentID, base)
	}
	if err != nil {
		return "", err
	}
	if prompt != "" {
		return prompt, nil
	}
	return base, nil
}

var _ outbound.SkillResolver = (*SkillResolver)(nil)
