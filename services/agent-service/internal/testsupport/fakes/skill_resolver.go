package fakes

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// SkillResolver returns a scripted prompt and records run-start resolutions.
type SkillResolver struct {
	mu      sync.Mutex
	Prompt  string
	Err     error
	Resolve func(context.Context, uuid.UUID, string, []domain.ToolSpec) (string, error)
	Calls   int
	// Specs records the tool specs of the last resolution so a test can assert the run
	// handed the resolver what it needs to rewrite a skill's tool references.
	Specs []domain.ToolSpec
}

// ResolveSystemPrompt returns Prompt when configured, otherwise the base prompt.
func (r *SkillResolver) ResolveSystemPrompt(ctx context.Context, agentID uuid.UUID, base string, specs []domain.ToolSpec) (string, error) {
	r.mu.Lock()
	r.Calls++
	r.Specs = specs
	resolve, prompt, err := r.Resolve, r.Prompt, r.Err
	r.mu.Unlock()
	if resolve != nil {
		return resolve(ctx, agentID, base, specs)
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
